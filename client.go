package simrpc

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"simrpc/codec"
	"sync"
)

// 只支持形如:
// func (t *T) MethodName(argType T1, replyType *T2) error
// 的rpc调用

// Call 一次rpc调用所需的所有信息
type Call struct {
	ServiceMethod string
	Seq           uint64
	Done          chan *Call // 为了实现异步的rpc调用
	Args          any
	Reply         any
	Error         error
}

func (call *Call) done() {
	call.Done <- call
}

// Client rpc客户端
type Client struct {
	cc       codec.Codec
	opt      *Option          // 一个conn只需要一个option
	header   *codec.Header    // 缓存一个header
	sending  sync.Mutex       // 保护请求的写入
	mu       sync.Mutex       // 保护client里的信息
	seq      uint64           // 一个client里标记不同call的自增id
	pending  map[uint64]*Call // 等待处理的call
	closing  bool             // 用户主动关闭
	shutdown bool             // 服务端要求关闭(很可能是出错了)
}

// ErrorShutdown 操作已经关闭了的客户端(close || shutdown)
var ErrorShutdown = errors.New("connection is shutdown")

// Close 用户主动关闭客户端
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closing == true {
		return ErrorShutdown
	}
	c.closing = true
	return c.cc.Close()
}

// IsAvailable 判断客户端是否关闭
func (c *Client) IsAvailable() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return !c.closing && !c.shutdown
}

// 在client中创建一个call
func (c *Client) registerCall(call *Call) (uint64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closing || c.shutdown {
		return 0, ErrorShutdown
	}

	call.Seq = c.seq
	c.pending[call.Seq] = call
	c.seq++
	return call.Seq, nil
}

// 在client中删除一个call
// 没有对应seq的call可以删除,则返回nil
func (c *Client) removeCall(seq uint64) *Call {
	c.mu.Lock()
	defer c.mu.Unlock()
	call, ok := c.pending[seq]
	if !ok {
		return nil
	}
	return call
}

// 遇到错误,取消掉所有后续的call
func (c *Client) terminateCalls(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sending.Lock()
	defer c.sending.Unlock()

	c.shutdown = true
	for _, call := range c.pending {
		call.Error = err
		call.done()
	}
}

// 接收服务端的返回
func (c *Client) receive() {
	var err error
	for err == nil {
		h := codec.Header{}
		err = c.cc.ReadHeader(&h)
		if err != nil {
			break
		}

		call := c.removeCall(h.Seq)
		switch {
		case call == nil:
			// 没有找到对应的call
			// 传输出错
			err = c.cc.ReadBody(nil) // 取出后续无用的body部分,避免影响下一次request读取
		case h.Error != "":
			// rpc调用结果中有err
			call.Error = errors.New(h.Error)
			err = c.cc.ReadBody(nil)
			call.done()
		default:
			// rpc调用成功
			err = c.cc.ReadBody(call.Reply)
			if err != nil {
				call.Error = fmt.Errorf("rpc client: read body error: %v", err)
			}
			call.done()
		}
	}
	// 结束接收过程
	c.terminateCalls(err)
}

func (c *Client) send(call *Call) {
	// 保证每次发送完整请求
	c.sending.Lock()
	defer c.sending.Unlock()

	seq, err := c.registerCall(call)
	if err != nil {
		call.Error = err
		call.done()
	}

	c.header.Seq = seq
	c.header.ServiceMethod = call.ServiceMethod
	c.header.Error = ""

	err = c.cc.Write(c.header, call.Args)
	if err != nil {
		// 写入失败就需要将注册的call删除
		call := c.removeCall(seq)

		// 写入失败的情况下又发现原来注册的call被receive删除了话
		// 这可能是因为write写入了一部分后才失败的
		// header部分是完整的所以这个call就在receive部分被处理掉了
		if call != nil {
			call.Error = err
			call.done()
		}
	}
}

// Go 异步rpc调用
func (c *Client) Go(serviceMethod string, args, reply any, done chan *Call) *Call {
	if done == nil {
		done = make(chan *Call, 1)
	} else if cap(done) == 0 {
		log.Panic("rpc client: done is a unbuffered channel\n")
	}

	call := &Call{
		ServiceMethod: serviceMethod,
		Done:          done,
		Args:          args,
		Reply:         reply,
		Error:         nil,
	}
	c.send(call)
	return call
}

// Call rpc同步调用
func (c *Client) Call(serviceMethod string, args, reply any) error {
	call := <-c.Go(serviceMethod, args, reply, make(chan *Call, 1)).Done
	return call.Error
}

// NewClient 创建Client实例
// 会完成option部分的发送以及接收协程的开启
func NewClient(conn net.Conn, option *Option) (*Client, error) {
	fn, ok := codec.NewCodecFuncMap[option.CodeType]
	if !ok {
		log.Printf("rpc client: invalid code type: %v\n", option.CodeType)
		err := fmt.Errorf("rpc client: invalid code type: %v", option.CodeType)
		return nil, err
	}

	// 发送option
	err := json.NewEncoder(conn).Encode(option)
	if err != nil {
		log.Printf("rpc client: encodeing option error: %v\n", err)
		_ = conn.Close()
		return nil, err
	}

	return NewClientCodec(fn(conn), option), nil
}

// NewClientCodec 初始化Client结构
func NewClientCodec(cc codec.Codec, option *Option) *Client {
	client := &Client{
		cc:       cc,
		opt:      option,
		header:   &codec.Header{},
		sending:  sync.Mutex{},
		mu:       sync.Mutex{},
		seq:      0,
		pending:  make(map[uint64]*Call),
		closing:  false,
		shutdown: false,
	}
	go client.receive()
	return client
}

func Dial(network, addr string, options ...*Option) (client *Client, err error) {
	option, err := parseOptions(options...)
	if err != nil {
		return nil, err
	}

	conn, err := net.Dial(network, addr)
	if err != nil {
		return nil, err
	}

	defer func() {
		// 创建失败记得关闭连接
		if client == nil {
			_ = conn.Close()
		}
	}()

	return NewClient(conn, option)
}

// 检查是否需要使用DefaultOption
func parseOptions(options ...*Option) (*Option, error) {
	// 传了空值
	if len(options) == 0 || options[0] == nil {
		return DefaultOption, nil
	}

	if len(options) > 1 {
		return nil, errors.New("rpc client: take more than one option")
	}

	option := options[0]
	option.MagicNumber = MagicNumber
	if option.CodeType == "" {
		option.CodeType = DefaultOption.CodeType
	}
	return option, nil
}
