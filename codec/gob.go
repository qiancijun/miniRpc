package codec

import (
	"bufio"
	"encoding/gob"
	"io"
	"log"
)

type GobCodec struct {
	conn io.ReadWriteCloser // conn 由构造函数传入，通常是通过 TCP 或者 Unix 建立 socket 得到的链接实例
	buf  *bufio.Writer      // 防止阻塞创建带缓冲的 Writer
	dec  *gob.Decoder
	enc  *gob.Encoder
}

func NewGobCodec(conn io.ReadWriteCloser) Codec {
	buf := bufio.NewWriter(conn)
	return &GobCodec{
		conn: conn,
		buf:  buf,
		dec:  gob.NewDecoder(conn),
		enc:  gob.NewEncoder(buf),
	}
}

var _ Codec = (*GobCodec)(nil)

// Close implements Codec.
func (g *GobCodec) Close() error {
	return g.conn.Close()
}

// ReadBody implements Codec.
func (g *GobCodec) ReadBody(body interface{}) error {
	return g.dec.Decode(body)
}

// ReadHeader implements Codec.
func (g *GobCodec) ReadHeader(header *Header) error {
	return g.dec.Decode(header)
}

// Write implements Codec.
func (g *GobCodec) Write(h *Header, body interface{}) error {
	defer func() {
		err := g.buf.Flush()
		if err != nil {
			_ = g.Close()
		}
	}()

	if err := g.enc.Encode(h); err != nil {
		log.Println("rpc codec: gob error encoding header: ", err)
		return err
	}

	if err := g.enc.Encode(body); err != nil {
		log.Println("rpc codec: gob error encoding body: ", err)
		return err
	}
	return nil
}