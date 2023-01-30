package codec

import "io"

// 一个完整request(请求)由一个header和一个body组成
// request的编码方式由建立连接时,连接首部的option决定

// Header 请求头
type Header struct {
	ServiceMethod string // 确定rpc调用什么方法:"Service.Method"
	Seq           uint64 // 标识一次rpc调用
	Error         string // 记录此次rpc调用是否成功
}

// Codec request的编解码器
// body部分预期是argv结构,包含一组参数
type Codec interface {
	io.Closer
	ReadHeader(*Header) error
	ReadBody(any) error
	Write(*Header, any) error
}

// NewCodecFunc Codec的构造函数
type NewCodecFunc func(writer io.ReadWriteCloser) Codec

// Type body选用的编解码方式
type Type string

// 列举可选的body编解码方式
const (
	GobType  = "gob"
	JsonType = "json"
)

var NewCodecFuncMap map[Type]NewCodecFunc

func init() {
	NewCodecFuncMap = make(map[Type]NewCodecFunc)
	NewCodecFuncMap[GobType] = NewGobCodec
	NewCodecFuncMap[JsonType] = NewJsonCodec
}
