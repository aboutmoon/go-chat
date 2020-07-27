package main

import (
	"fmt"
	"io"
	"net"
	"runtime"
	"sync"
)

// 创建全局的client结构体
type Client struct {
	Name string
	Addr string
	C chan string
}

// 创建全局读写锁，保护公共区数据
var rwMutex sync.RWMutex

// 创建全局在线用户列表
var onlineMap = make(map[string]Client)

// 创建全局message
var message = make(chan string)

func main()  {
	// 启动器，监听客户端的连接请求
	listener, err := net.Listen("tcp", "127.0.0.1:8800")
	if err != nil {
		fmt.Println("listen err: ", err)
	}
	defer listener.Close()

	// 创建并启动Manager管理者go程
	go Manager()

	// 服务器循环监听客户端连接请求
	for  {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Accept err:", err)
			continue
		}
		go handleConnect(conn)
	}
}

// 负责对接客户端数据通信
func handleConnect(conn net.Conn) {
	defer conn.Close()

	// 获取客户端地址结构
	ctlAddr := conn.RemoteAddr().String()

	ctl := Client{ctlAddr, ctlAddr, make(chan string)}

	// 添加到在线用户列表
	onlineMap[ctlAddr] = ctl

	// 启动 go程 读用户自己的 channel中的数据
	rwMutex.Lock()
	go WriteMsgToClient(ctl, conn)
	rwMutex.Unlock()

	// 组织用户上线广播消息 （IP+port）Name： 上线！
	message <- makeMsg("上线。", ctl)

	// 在用户广播自己上线后，创建go程，获取用户聊天内容
	go func() {

		for {
			buf := make([]byte, 4096)
			// 读取用户发送的消息
			n, err := conn.Read(buf) // 阻塞
			if n == 0 {
				fmt.Println("客户端下线！")
			}
			if err != nil && err != io.EOF {
				fmt.Println("Read err:", err)
				return
			}

			msg := string(buf[:n -1])

			// 判断，用户是否发送的是一个查询在线用户命令
			if "who" == msg {
				rwMutex.RLock()

				for _, ctl := range onlineMap {
					// 将在线用户，组织成消息，发送给当前用户
					onlineMsg := makeMsg("[在线]!\n", ctl)

					conn.Write([]byte(onlineMsg))
				}

				rwMutex.RUnlock()
			} else {
				message <- makeMsg( msg , ctl)
			}

		}
	}()
	
	// 添加测试代码，防止当前go程提前退出
	for  {
		runtime.GC()
	}

}

func makeMsg(str string, ctl Client) string  {
	return "[" + ctl.Addr + "]" + ctl.Name + ":  " + str
}

func WriteMsgToClient(ctl Client, conn net.Conn) {
	for  {
		msg := <- ctl.C // 由于make(chan string)创建的channel默认是阻塞的，所以如果没有读取到值，则不会执行后续语句

		// 将读到的数据写入对应的客户端
		conn.Write([]byte(msg + "\n"))
	}
}

// 全局的Manager go程， 监听全局message中是否有数据
func Manager()  {
	for  {
		msg := <- message //无数据，阻塞；有数据，继续

		rwMutex.RLock() // 读锁
		// 实时的遍历在线用户列表
		for _, ctl := range onlineMap{
			ctl.C <- msg
		}
		rwMutex.RUnlock() // 读锁解锁
	}
}




