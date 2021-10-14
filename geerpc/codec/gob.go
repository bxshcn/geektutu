package codec

import (
	"bufio"
	"encoding/gob"
	"io"
	"log"
)

// gob实现Codec接口
// 接口只是行为，实现接口的结构体必须操作一定的数据对象，这样结构体才有现实意义
// 编解码首先要针对一个流，我们用
type GobCodec struct {
	conn   io.ReadWriteCloser
	buf    *bufio.Writer // 根据conn创建一个缓冲，避免写阻塞，提高效率
	dec    *gob.Decoder
	enc    *gob.Encoder
	closed bool
}

func NewGobCodec(rwc io.ReadWriteCloser) Codec {
	buf := bufio.NewWriter(rwc)
	return &GobCodec{
		conn: rwc,
		buf:  buf,
		dec:  gob.NewDecoder(rwc),
		enc:  gob.NewEncoder(buf),
	}
}

func (c *GobCodec) ReadHeader(header *Header) error {
	return c.dec.Decode(header)
}

// 这个body必须是一个分配了空间的指针对象
func (c *GobCodec) ReadBody(body interface{}) error {
	return c.dec.Decode(body)
}

// writer header first, and then body
// 这里的body是一个非指针对象
func (c *GobCodec) Write(header *Header, body interface{}) (err error) {
	if err = c.enc.Encode(*header); err != nil {
		log.Println("rpc: gob error encoding header:", err)
		// 有没可能c.buf.Flush失败呢？除非底层的连接已经断开，否则不会失败
		// 而如果连接已经断开，那在断开的时候就已经会给出报错了，不用这里给出报错
		if c.buf.Flush() == nil {
			//log.Println("rpc: gob error encoding header:", err)
			c.Close()
		}
		return
	}
	if err = c.enc.Encode(body); err != nil {
		log.Println("rpc: gob error encoding body:", err)
		if c.buf.Flush() == nil {
			c.Close()
		}
		return
	}
	return c.buf.Flush()
}

func (c *GobCodec) Close() error {
	if c.closed {
		return nil
	}
	c.closed = true
	return c.conn.Close()
}
