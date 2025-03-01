MemTable 实现了 Redis 6.0 之前大部分功能，在项目中，你可以学习到以下知识：

- Redis 服务器的基础框架以及各种数据结构；
- 精进 Go 语言的掌握程度；
- 了解 Lua 虚拟机是如何运行的；
- 交互式命令行的简单实现；
- 从性能优化出发，学习操作系统以及网络的相关知识。

内存数据库，或者缓存服务，涉及到的知识范围非常广，如果你打算从零开始做一个类似的项目，那么你还能够收获更多。

## 数据结构

MemTable 使用了不同的数据结构来解决不同场景的问题：

- list：用于时间事件链表等频繁动态读写的场景；
- capped_list：用于慢查询日志等限制缓存上限的场景；
- skiplist：用于范围性顺序查找的场景；
- trie_tree：用于单词查询、路径查询等场景；
- ring_buffer：用于限制缓存上限的字符串存储；
- link_buffer：用于日志缓冲等需要读写分离、控制读写速率的场景。

选取一个合适的数据结构作为数据容器，能够更加轻松地解决某些场景下的问题。Linux Kernel 中的许多精妙设计，都是基于数据结构的选择。

## 消息通知机制

MemTable 中使用了同步与异步两种消息通知机制。借助 Golang 中的 select 与 channel 机制，异步通信更加简洁和高效，能够更加轻松地实现不同协程之间的通信，软件的不同模块也可以使用通信机制的弱耦合来代替调用的强耦合，从而使软件的模块化更加明显。使用 channel 还能够实现类似回调函数的机制，减少事务线程的工作。

## 操作系统

MemTable 是一个内存数据库服务，涉及到了网络、硬盘、调度等多方面的知识。通过 pprof 对软件性能进行测试，你能够发现 MemTable 设计和实现上的不足，如 GC 机制、goroutine 调度。在性能调优的过程中，