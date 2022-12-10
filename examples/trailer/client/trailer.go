package main

import (
	"context"
	"fmt"
	"github.com/cloudwego/hertz/pkg/app/client"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"io"
)

func main() {
	c, _ := client.NewClient(client.WithResponseBodyStream(true))
	req := &protocol.Request{}
	resp := &protocol.Response{}
	req.SetMethod(consts.MethodGet)
	req.SetRequestURI("http://127.0.0.1:8080/trailer")
	err := c.Do(context.Background(), req, resp)
	if err != nil {
		return
	}
	bodyStream := resp.BodyStream()

	buf := make([]byte, 5)
	for true {
		n, err := bodyStream.Read(buf)
		if err != nil {
			if err == io.EOF {
				break
			}
			panic(err)
		}
		fmt.Printf(string(buf[:n]))
	}

	resp.Header.VisitAllTrailer(visitSingle)

	fmt.Println(string(resp.Header.TrailerHeader()))

}

func visitSingle(k []byte) {
	fmt.Printf("%s\n", string(k))
}
