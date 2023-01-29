package codec

import (
	"bufio"
	"encoding/json"
	"io"
	"log"
)

// json编码和gob编码都是通过标准库实现的
// 标准库提供的接口也很统一
// 完全就是什么都不用改然后gob变成json就实现了JsonCodec

// JsonCodec Codec的Json编码实现
type JsonCodec struct {
	conn io.ReadWriteCloser
	buf  *bufio.Writer
	dec  *json.Decoder
	enc  *json.Encoder
}

func NewJsonCodec(conn io.ReadWriteCloser) Codec {
	buf := bufio.NewWriter(conn) // 加一个缓冲区
	return JsonCodec{
		conn: conn,
		buf:  buf,
		dec:  json.NewDecoder(conn),
		enc:  json.NewEncoder(buf),
	}
}

func (g JsonCodec) Close() error {
	return g.conn.Close()
}

func (g JsonCodec) ReadHeader(header *Header) error {
	return g.dec.Decode(header)
}

func (g JsonCodec) ReadBody(body any) error {
	return g.dec.Decode(body)
}

func (g JsonCodec) Write(header *Header, body any) error {
	defer func() {
		_ = g.buf.Flush()
		_ = g.Close()
	}()

	if err := g.enc.Encode(header); err != nil {
		log.Printf("rpc encoding header error: %v\n", err)
		return err
	}

	if err := g.enc.Encode(body); err != nil {
		log.Printf("rpc encoding body error: %v\n", err)
		return err
	}

	return nil
}
