package main

import (
	"fmt"
	"log"
	"net"
	"sync"
	"time"
)

func main() {

	sAddr := "0.0.0.0:55442" // "0.0.0.0:55442"
	adr, err := net.ResolveUDPAddr("udp", sAddr)

	if err != nil {
		log.Println(err)
	}

	Connections := SafeConnections{c: make(typeConnMap)}

	requests := make(chan sms, 5)

	Listener, err := net.ListenUDP("udp", adr)
	if err != nil {
		log.Println(err)
	}

	fmt.Println("Server started with :", sAddr, " Address")

	settings := serverSetting{
		30 * time.Second,
		30 * time.Second,
		60 * time.Second,
	}
	Connections.GC(settings)
	go Sender(Listener, &Connections, requests)

	buff := make([]byte, 1400)

	for {

		_, userAddr, _ := Listener.ReadFromUDP(buff)

		Connections.CheckAndAdd(userAddr)

		requests <- sms{
			uAddr:   userAddr,
			content: buff,
		}

	}

}

func Sender(Listener *net.UDPConn, Cons *SafeConnections, requests chan sms) {
	for request := range requests {
		Cons.Lock()
		for _, user := range Cons.c {
			if user.uAddr.String() != request.uAddr.String() {
				_, err := Listener.WriteTo(
					request.content,
					user.uAddr,
				)
				if err != nil {
					log.Println(err)
				}
			}
		}
		Cons.Unlock()
	}
}

type sms struct {
	uAddr   *net.UDPAddr
	content []byte
}

type serverSetting struct {
	checkTime time.Duration
	offlineTime time.Duration
	removeTime time.Duration
}

func (s *serverSetting) getOfflineTime() int64 {
	return int64(s.offlineTime)
}

func (s *serverSetting) getRemoveTime() int64 {
	return int64(s.removeTime)
}

type connMap struct {
	uAddr            *net.UDPAddr
	status           bool
	lastMessagesTime time.Time
}

type typeConnMap map[string]*connMap

type SafeConnections struct {
	sync.RWMutex
	c typeConnMap
}

func (m *SafeConnections) CheckAndAdd(uAddr *net.UDPAddr) {
	m.Lock()

	if _, Is := m.c[uAddr.String()]; Is {
		m.c[uAddr.String()].lastMessagesTime = time.Now()
		m.c[uAddr.String()].status = true
	} else {
		user := connMap{
			uAddr,
			true,
			time.Now(),
		}
		m.c[uAddr.String()] = &user
		fmt.Println("Add new user: ", uAddr.String(), m.c[uAddr.String()])
	}

	m.Unlock()
}

func (m *SafeConnections) GC(s serverSetting) {
	go func() {
		ticker := time.NewTicker(s.checkTime)
		for range ticker.C {
			m.Lock()
			for k, _ := range m.c {
				if time.Now().Unix()-m.c[k].lastMessagesTime.Unix() > s.getOfflineTime() {
					m.c[k].status = false
				}

				if time.Now().Unix()-m.c[k].lastMessagesTime.Unix() > s.getRemoveTime() && m.c[k].status == false {
					fmt.Println("Remove users: ", k)
					delete(m.c, k)
				}
			}
			m.Unlock()
		}
	}()
}
