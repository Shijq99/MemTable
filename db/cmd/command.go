package cmd

import (
	"MemTable/db"
	"MemTable/resp"
)

type Command = func(base *db.DataBase, cmd [][]byte) resp.RedisData

var CommandTable = make(map[string]Command)

func RegisterCommand(name string, cmd Command) {
	CommandTable[name] = cmd
}

func init() {

	println("cmd init")

	RegisterStringCommands()
	RegisterSetCommand()

}

func ExecCommand(base *db.DataBase, cmd [][]byte) resp.RedisData {

	if len(cmd) == 0 {
		return resp.MakeErrorData("error: empty command")
	}

	f, ok := CommandTable[string(cmd[0])]
	if !ok {
		return resp.MakeErrorData("error: unsupported command")
	}

	return f(base, cmd)
}
