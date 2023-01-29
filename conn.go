package simpleRPC

import "simrpc/codec"

// 与rpc连接相关的结构定义

// MagicNumber 随便一个数用来对协议正确性
const MagicNumber = 0x3bef5c

// Option 通信选项
// 方便起见option会直接选用json作为编解码方式
// 正常而言,协议应当设计好前几个比特中那几位标记option的编解码方式option长度等
// 这样则可以对option也支持不同的编码方式,提升灵活度
// | Option | Header1 | Body1 | Header2 | Body2 | ...
type Option struct {
	MagicNumber int
	CodeType    codec.Type
}

var DefaultOption = &Option{
	MagicNumber: MagicNumber,
	CodeType:    codec.GobType,
}
