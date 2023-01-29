package codec

import (
	"bufio"
	"encoding/gob"
	"io"
	"log"
)

// GobCodec Codec的Gob编码实现
type GobCodec struct {
	conn io.ReadWriteCloser
	buf  *bufio.Writer
	dec  *gob.Decoder
	enc  *gob.Encoder
}

func NewGobCodec(conn io.ReadWriteCloser) Codec {
	buf := bufio.NewWriter(conn) // 加一个缓冲区
	return GobCodec{
		conn: conn,
		buf:  buf,
		dec:  gob.NewDecoder(conn),
		enc:  gob.NewEncoder(buf),
	}
}

func (g GobCodec) Close() error {
	return g.conn.Close()
}

func (g GobCodec) ReadHeader(header *Header) error {
	return g.dec.Decode(header)
}

func (g GobCodec) ReadBody(body any) error {
	return g.dec.Decode(body)
}

func (g GobCodec) Write(header *Header, body any) error {
	defer func() {
		_ = g.buf.Flush()
		_ = g.Close()
	}()

	if err := g.enc.Encode(header); err != nil {
		log.Printf("rpc codec: encoding header error: %v\n", err)
		return err
	}

	if err := g.enc.Encode(body); err != nil {
		log.Printf("rpc codec: encoding body error: %v\n", err)
		return err
	}

	return nil
}
