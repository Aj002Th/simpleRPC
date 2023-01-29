package codec

import "io"

// Header 请求头
// 方便起见header会直接选用json作为编解码方式
// 正常而言,协议应当设计好前几个比特中那几位标记header的编解码方式、header长度等
// 这样则可以对header也支持不同的编码方式,提升灵活度
type Header struct {
	ServiceMethod string // 确定rpc调用什么方法:"Service.Method"
	Seq           uint64 // 标识一次rpc调用
	Error         string // 记录此次rpc调用是否成功
}

// Codec 请求体的编解码器
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
