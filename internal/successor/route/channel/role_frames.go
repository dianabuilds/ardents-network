package channel

import (
	"errors"
	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
	"net"
)

func SendHello(conn net.Conn, h ardp.Hello) error {
	body, err := ardp.EncodeHello(h)
	if err != nil {
		return err
	}
	return ardp.WriteFrame(conn, ardp.Frame{Kind: ardp.KindHello, Body: body})
}
func ReadHello(conn net.Conn) (ardp.Hello, error) {
	f, err := ardp.ReadFrame(conn)
	if err != nil {
		return ardp.Hello{}, err
	}
	if f.Kind != ardp.KindHello || f.Lane != 0 {
		return ardp.Hello{}, errors.New("route HELLO required")
	}
	return ardp.DecodeHello(f.Body)
}
func Accept(conn net.Conn) error {
	f, err := ardp.AcceptFrame(0, Window)
	if err != nil {
		return err
	}
	return ardp.WriteFrame(conn, f)
}
func Accepted(conn net.Conn) error {
	f, err := ardp.ReadFrame(conn)
	if err != nil {
		return err
	}
	status, credit, err := ardp.DecodeAcceptFrame(f)
	if err != nil || status != 0 || credit != Window {
		return errors.Join(errors.New("route admission refused"), err)
	}
	return nil
}
