package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"pnt/api/controller/node"
	"pnt/api/pkg/e"
	"pnt/client/request"

	"github.com/urfave/cli"
)

func format(res *request.Response, err error) {
	if err != nil {
		fmt.Printf("\033[40;31m %s \033[0m", err.Error())
		return
	}
	if res.Code != e.SUCCESS {
		fmt.Printf("Error msg is %s", res.Msg)
		return
	}
	var str bytes.Buffer
	data, err := json.Marshal(res.Data)
	if err != nil {
		fmt.Printf("\033[40;31m Error json Marshal %s \033[0m", err.Error())
		return
	}
	json.Indent(&str, data, "", "    ")

	if str.Len() > 2 {
		fmt.Printf("\033[40;32m %s \033[0m", str.String())
	}
}

func Node() cli.Command {
	return cli.Command{
		Name:    "node",
		Aliases: []string{"n"},
		Usage:   "exe cmd to node cache ",
		Subcommands: []cli.Command{
			{
				Name:     "get",
				Usage:    "get node cache by id or all",
				Category: "node",
				Action: func(c *cli.Context) {
					nid := c.Args().First()
					if nid == "" {
						format(request.Get("/api/v1/node"))
					} else {
						format(request.Get("/api/v1/node/" + nid))
					}
				},
			},
			{
				Name:     "add",
				Usage:    `add a new node cache need input json string. example: '{"node_info":{"node_id":"","alias":"", "vps_addr":"","c_nat_addr":"","d_net_addr":"","region":"","work_status":0,"roler":0,"control_port":0,"data_tunn_port":0,"fresh_interval":0,"provide_list":null},"hardware":{"bandwidth":0,"cpu":null,"GPU":null,"disk":null},"limits":{"max_connection":0,"speed":0}}'`,
				Category: "node",
				Action: func(c *cli.Context) error {
					input := c.Args().First()
					if input == "" {
						fmt.Printf("Error: %s", `Enter the node example: '{"node_info":{"node_id":"","alias":"", "vps_addr":"","c_nat_addr":"","d_net_addr":"","region":"","work_status":0,"roler":0,"control_port":0,"data_tunn_port":0,"fresh_interval":0,"provide_list":null},"hardware":{"bandwidth":0,"cpu":null,"GPU":null,"disk":null},"limits":{"max_connection":0,"speed":0}}'`)
						return nil
					}
					format(request.Post("/api/v1/node", input))
					return nil
				},
			},
			{
				Name:     "set",
				Usage:    "set a node cache attribute by node id",
				Category: "node",
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:  "node_id,n",
						Usage: "set node id",
					},
					cli.StringFlag{
						Name:  "key,k",
						Usage: "set node key",
					},
					cli.StringFlag{
						Name:  "field,f",
						Usage: "set node field",
					},
					cli.StringFlag{
						Name:  "val,v",
						Usage: "set node val",
					},
				},
				Action: func(c *cli.Context) {
					nid := c.String("node_id")
					k := c.String("key")
					f := c.String("field")
					v := c.String("val")
					if nid == "" || k == "" || f == "" || v == "" {
						fmt.Printf("Error: %s", "Incomplete parameters please run `node set -h`")
						return
					}
					body := &node.SetCacheParams{Key: k, FieldName: f, Value: v}
					params, err := json.Marshal(body)
					if err != nil {
						fmt.Println(err.Error())
						return
					}
					format(request.Put("/api/v1/captcha/"+nid, string(params[:])))
				},
			},
			{
				Name:     "del",
				Usage:    "delete node cache by node id",
				Category: "node",
				Action: func(c *cli.Context) {
					nid := c.Args().First()
					if nid == "" {
						fmt.Printf("Error: %s", "Enter the node ID to delete")
						return
					}
					format(request.Delete("/api/v1/captcha/" + nid))
				},
			},
			{
				Name:     "self",
				Usage:    "get self node cache",
				Category: "node",
				Action: func(c *cli.Context) {
					format(request.Get("/api/v1/self/node"))
				},
			},
		},
	}
}

func Logs() cli.Command {
	return cli.Command{
		Name:  "log",
		Usage: "Read log cache",
		Action: func(c *cli.Context) {
			lines := c.Args().First()
			if lines == "" {
				format(request.Get("/api/v1/log"))
			} else {
				format(request.Get("/api/v1/log/" + lines))
			}
		},
	}
}

func Proxy() cli.Command {
	return cli.Command{
		Name:  "proxy",
		Usage: "Proxy Browser Access",
		Action: func(c *cli.Context) {
			format(request.Post("/api/v1/proxy/request", fmt.Sprintf(`{"path": "%s"}`, c.Args().First())))
		},
	}
}

func Kerrigan() cli.Command {
	return cli.Command{
		Name:  "stop",
		Usage: "stop kerrigan server",
		Action: func(c *cli.Context) {
			format(request.Post("/api/v1/kerrigan/stop", ""))
		},
	}
}

func GetClientCommands() []cli.Command {
	var cmd []cli.Command
	cmd = append(cmd, Node())
	cmd = append(cmd, Logs())
	cmd = append(cmd, Proxy())
	cmd = append(cmd, Kerrigan())
	return cmd
}
