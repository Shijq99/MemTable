package server

import (
	"fmt"
	"github.com/tangrc99/MemTable/config"
	"github.com/tangrc99/MemTable/db"
	"github.com/tangrc99/MemTable/logger"
	"github.com/tangrc99/MemTable/resp"
	"github.com/tangrc99/MemTable/utils/gopool"
	"net"
	"os"
	"os/signal"
	"path"
	"syscall"
	"time"
)

type Server struct {
	url      string       // 监听 url
	listener net.Listener // listener
	dir      string       // 工作目录

	// 数据库部分
	dbs   []*db.DataBase // 多个可以用于切换的数据库
	Chs   *db.Channels   // 订阅发布频道
	dbNum int            //数据库数量

	// 客户端部分
	clis       *ClientList  // 客户端列表
	cliTimeout int          // 客户端失效时间
	maxClients int          // 最大客户端数量
	events     chan *Client // 用于解析完毕的协程同步

	tl *TimeEventList // 时间事件链表

	// 退出控制
	quit     bool
	quitFlag chan struct{}

	// 持久化
	rdbFile    string     // rdb 文件名
	dirty      int        // 脏数据计数器
	checkPoint int64      // rdb 时间
	RDBStatus             // rdb 文件状态
	aofFile    string     // aof 文件名
	aof        *AOFBuffer // aof 缓冲区
	aofEnabled bool       // 是否开启 aof

	// 协程池
	gopool *gopool.Pool // 用于客户端启动的协程池
	sts    *Status

	// 主从复制
	ReplicaStatus
}

func NewServer(url string) *Server {

	d := make([]*db.DataBase, config.Conf.DataBases)
	for i := 0; i < config.Conf.DataBases; i++ {
		d[i] = db.NewDataBase()
	}

	s := &Server{
		dbs:        d,
		dbNum:      config.Conf.DataBases,
		Chs:        db.NewChannels(),
		clis:       NewClientList(),
		tl:         NewTimeEventList(),
		url:        url,
		events:     make(chan *Client, 10000),
		quit:       false,
		quitFlag:   make(chan struct{}),
		rdbFile:    config.Conf.RDBFile,
		sts:        NewStatus(),
		cliTimeout: config.Conf.Timeout,
		maxClients: config.Conf.MaxClients,
		dir:        config.Conf.Dir,
		aofEnabled: config.Conf.AppendOnly,
		aofFile:    "appendonly.aof",
	}
	return s
}

func (s *Server) InitModules() {
	// aof 开关
	if config.Conf.AppendOnly {
		logger.Debug("Config: AppendOnly Enabled")
		s.aof = NewAOFBuffer(config.Conf.Dir + "appendonly.aof")

	}

	if config.Conf.GoPool {
		s.gopool = gopool.NewPool(config.Conf.GoPoolSize, 0, config.Conf.GoPoolSpawn)
		logger.Debug("Config: GoPool Enabled")
	}
}

func (s *Server) handleRead(conn net.Conn) {

	client := NewClient(conn)

	ch := resp.ParseStream(conn)

	//pipelined := 0

	// 这里会阻塞等待有数据到达
	running := true

	for running && !s.quit {

		select {
		// 等待是否有新消息到达
		case parsed := <-ch:

			if parsed.Err != nil {

				if e := parsed.Err.Error(); e == "EOF" {
					logger.Debug("Client", client.id, "Peer ShutDown Connection")
				} else {
					logger.Debug("Client", client.id, "Read Error:", e)
				}
				running = false
				break
			}

			if plain, ok := parsed.Data.(*resp.PlainData); ok {

				client.pipelined = true
				client.cmd = plain.ToCommand()
				client.raw = parsed.Data.ToBytes()

			} else if array, ok := parsed.Data.(*resp.ArrayData); ok {

				client.cmd = array.ToCommand()
				client.raw = parsed.Data.ToBytes()

			} else {

				logger.Warning("Client", client.id, "parse Command Error")
				running = false
				break
			}

			// 如果解析完毕有可以执行的命令，则发送给主线程执行
			s.events <- client

			// 使用 select 防止协程无法释放
			select {

			case <-client.exit:
				running = false

			case r := <-client.res:

				// 将主线程的返回值写入到 socket 中
				_, err := conn.Write((*r).ToBytes())

				if err != nil {
					logger.Warning("Client", client.id, "Write Error")
					running = false
					break
				}
			}

			client.pipelined = false

		case <-client.exit:
			running = false

		case msg := <-client.msg:
			// 写入发布订阅消息
			_, err := conn.Write(msg)

			if err != nil {
				logger.Warning("Client", client.id, "Write Error")
				running = false
				break
			}
		}
	}

	// 如果是读写发生错误，需要通知事件循环来关闭连接
	if client.status != EXIT {
		// 说明这是异常退出的
		client.status = ERROR
		client.cmd = nil

		// 通知顶层
		s.events <- client
	}

	err := conn.Close()
	if err != nil {
		return
	}

	logger.Info("Client Shutdown", conn.RemoteAddr().String())

}

func (s *Server) eventLoop() {

	s.initTimeEvents()
	timer := time.NewTimer(100 * time.Millisecond)

	for !s.quit {

		select {

		case <-timer.C:

			timer.Reset(100 * time.Millisecond)

			logger.Debug("EventLoop: Timer trigger")
			// 需要完成定时任务
			s.tl.ExecuteManyDuring(s.sts.Now, 25*time.Millisecond)
			//s.tl.ExecuteOneIfExpire()

		case cli := <-s.events:
			logger.Debug("EventLoop: New Event From Client", cli.id.String())

			// todo:  关闭使用定时队列来实现
			// 底层发生异常，需要关闭客户端，或者客户端已经关闭了，那么就不处理请求了
			if cli.status == ERROR || cli.status == EXIT {
				// 释放客户端资源
				logger.Debug("EventLoop: Remove Closed Client", cli.id.String())
				cli.UnSubscribeAll(s.Chs)
				s.clis.RemoveClient(cli)
				continue
			}

			// 用于判断是否为新连接
			ok := s.clis.AddClientIfNotExist(cli)

			// 如果是新连接
			if ok {
				logger.Debug("EventLoop: New Client", cli.id.String())
			}

			// 更新时间戳
			cli.UpdateTimestamp(s.sts.Now)

			// 执行命令
			res, isWriteCommand := ExecCommand(s, cli, cli.cmd)

			// 只有写命令需要完成aof持久化
			if isWriteCommand && fmt.Sprintf("%T", res) != "*resp.ErrorData" {

				if cli.pipelined {
					cli.raw = resp.PlainDataToResp(cli.cmd).ToBytes()
				}

				s.appendAOF(cli)
				s.updateReplicaStatus(cli)
				s.dirty++
			}

			// 非阻塞状态的客户端写入回包
			if !cli.blocked {
				cli.res <- &res
			}

		}
	}

	// 处理退出逻辑
	logger.Info("Server: Ready To Shutdown")

	// 关闭监听
	_ = s.listener.Close()

	// 进行数据持久化
	s.saveData()

	// 关闭所有的客户端协程
	for s.clis.Size() != 0 {
		front := s.clis.list.FrontNode()
		s.clis.removeClientWithPosition(front.Value.(*Client), front)
		// 不用删除订阅
	}

	// 通知
	s.quitFlag <- struct{}{}
}

func (s *Server) acceptLoop() {

	logger.Info("Server: Start Listen")

	for !s.quit {
		conn, err := s.listener.Accept()
		if err != nil {
			break
		}

		// 如果客户端数量过多，拒绝服务
		if s.maxClients > 0 && s.clis.Size() >= s.maxClients {
			_ = conn.Close()
		}

		if s.gopool != nil {

			s.gopool.Schedule(func() {
				s.handleRead(conn)
			})

		} else {
			go s.handleRead(conn)
		}

	}
	logger.Info("Server: Shutdown Listener")

}

func (s *Server) initTimeEvents() {

	// 每 300 秒清理一次过期客户端
	s.tl.AddTimeEvent(NewPeriodTimeEvent(func() {
		logger.Debug("TimeEvent: Remove Inactive Clients")

		// 如果设置过期值小于 0 则不需要进行清理
		if s.cliTimeout < 0 {
			return
		}

		s.clis.RemoveLongNotUsed(3, 20, time.Duration(s.cliTimeout)*time.Second)

	}, time.Now().Add(10*time.Second).Unix(), 10*time.Second,
	))

	// 过期 key 清理
	s.tl.AddTimeEvent(NewPeriodTimeEvent(func() {
		logger.Debug("TimeEvent: Remove Expired Keys")

		for _, dataBase := range s.dbs {
			// 抽样 20 个，如果有 5 个过期，则再次删除
			for dataBase.CleanTTLKeys(20) >= 5 {
			}
		}

	}, time.Now().Add(time.Second).Unix(), time.Second,
	))

	// AOF 刷盘
	s.tl.AddTimeEvent(NewPeriodTimeEvent(func() {
		logger.Debug("TimeEvent: AOF FLUSH")

		if s.aofEnabled {
			s.aof.Flush()
		}
		//s.aof.Sync()

	}, time.Now().Add(time.Second).Unix(), time.Second,
	))

	// bgsave 持久化 trigger
	s.tl.AddTimeEvent(NewPeriodTimeEvent(func() {
		logger.Debug("TimeEvent: RDB Check")

		if !s.aofEnabled && (s.dirty > 100 || s.sts.Now.Unix()-s.checkPoint > 10) {
			s.BGRDB()
		}

	}, time.Now().Add(time.Second).Unix(), time.Second,
	))

	// 更新服务端信息
	s.tl.AddTimeEvent(NewPeriodTimeEvent(func() {
		logger.Debug("TimeEvent: Update Status")

		s.sts.UpdateStatus()

	}, time.Now().Add(time.Second).Unix(), time.Second,
	))

	// 主从复制相关操作
	s.tl.AddTimeEvent(NewPeriodTimeEvent(func() {
		logger.Debug("TimeEvent: Replication")

		s.handleReplicaEvents()

	}, time.Now().Add(200*time.Millisecond).Unix(), 200*time.Millisecond,
	))
}

func (s *Server) Start() {

	// 初始化操作

	logger.Info("Server: Listen at", s.url)

	go s.eventLoop()

	// 开启监听
	var err error
	s.listener, err = net.Listen("tcp", s.url)
	if err != nil {
		logger.Error("Server:", err.Error())
	}

	go s.acceptLoop()

	quit := make(chan os.Signal)

	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM) // 接受软中断信号并且传递到 channel

	<-quit

	// 通知主线程在完成任务后退出，防止有任务进行到一半
	s.quit = true
	<-s.quitFlag

	logger.Info("Server Shutdown...")
}

func (s *Server) TryRecover() {

	aof := path.Join(s.dir, s.aofFile)
	rdb := path.Join(s.dir, s.rdbFile)

	if _, err := os.Stat(aof); err == nil {
		logger.Info("Recover From AppendOnly File")
		s.recoverFromAOF(aof)
	} else if _, err := os.Stat(aof); err == nil {
		logger.Info("Recover From RDB File")
		s.recoverFromRDB(aof, rdb)
	}

}

func (s *Server) saveData() {

	// 优先使用 aof 进行存储
	if s.aofEnabled && s.aof != nil {

		s.aof.Quit()

	} else {

		ok := s.RDB(path.Join(s.dir, s.rdbFile))
		if !ok {
			logger.Error("Quit: Generate RDB File Failed")
		} else {
			logger.Info("Quit: Generated RDB File")
		}
	}
}
