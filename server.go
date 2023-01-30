package simrpc

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"reflect"
	"simrpc/codec"
	"sync"
)

// Server rpc服务端
type Server struct{}

func NewServer() *Server {
	return &Server{}
}

var DefaultServer = NewServer()

// Accept 使用默认server进行连接建立
func Accept(listener net.Listener) {
	DefaultServer.Accept(listener)
}

// Accept 处理rpc客户端建立连接的请求
func (s *Server) Accept(listener net.Listener) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("rpc server: accept error: %v\n", err)
			return
		}
		go s.serveConn(conn)
	}
}

// 处理连接开头的Option部分
func (s *Server) serveConn(conn io.ReadWriteCloser) {
	defer func() {
		_ = conn.Close()
	}()

	option := &Option{}
	err := json.NewDecoder(conn).Decode(option)
	if err != nil {
		log.Printf("rpc server: option decode error: %v\n", err)
		return
	}

	if option.MagicNumber != MagicNumber {
		log.Printf("rpc server: invalid magic number: %v\n", option.MagicNumber)
		return
	}

	fn, ok := codec.NewCodecFuncMap[option.CodeType]
	if !ok {
		log.Printf("rpc server: invalid code type: %v\n", option.CodeType)
		return
	}

	s.serveCodec(fn(conn))
}

var invalidReply = struct{}{}

// 处理后续的一连串request(header + body)部分
func (s *Server) serveCodec(cc codec.Codec) {
	defer func() {
		_ = cc.Close()
	}()

	sending := &sync.Mutex{}
	wg := &sync.WaitGroup{} // 用于等待所有request都被处理完

	for {
		req, err := s.readRequest(cc)
		if err != nil {
			if req == nil {
				// error塞不回去了,这个连接有问题
				break
			}
			// 将error带回给
			req.header.Error = err.Error()
			s.sendResponse(cc, req.header, invalidReply, sending)
			continue
		}
		wg.Add(1)
		go s.handleRequest(cc, req, sending, wg)
	}
	wg.Wait()
}

func (s *Server) readRequest(cc codec.Codec) (*request, error) {
	h := codec.Header{}
	err := cc.ReadHeader(&h)
	if err != nil {
		log.Printf("rpc server: read header fail: %v\n", err)
		return nil, err
	}

	req := &request{
		header: &h,
		argv:   reflect.Value{},
		reply:  reflect.Value{},
	}

	// 还没有定义request body具体的类型,先用string代替
	req.argv = reflect.New(reflect.TypeOf(""))

	err = cc.ReadBody(req.argv.Interface())
	if err != nil {
		log.Printf("rpc server: read body fail: %v\n", err)
		return nil, err
	}

	return req, nil
}

func (s *Server) sendResponse(cc codec.Codec, header *codec.Header, body any, sending *sync.Mutex) {
	sending.Lock()
	defer sending.Unlock()

	err := cc.Write(header, body)
	if err != nil {
		log.Printf("rpc server: send response fail: %v\n", err)
	}
}

func (s *Server) handleRequest(cc codec.Codec, req *request, sending *sync.Mutex, wg *sync.WaitGroup) {
	// 这里需要做的是依据header中指定的ServiceMethod找到并调用相应的方法
	// 然后把返回值放到body里发送回客户端

	defer wg.Done()

	// 还没有确定response body的具体类型,先用string代替
	req.reply = reflect.ValueOf(fmt.Sprintf("rpc reply: %v", req.header.Seq))

	// 先只实现一个服务端的简单打印
	log.Printf("[rpc server] get request: %v - %v\n", req.header, req.argv.Elem())
	s.sendResponse(cc, req.header, req.reply.Interface(), sending)
}
