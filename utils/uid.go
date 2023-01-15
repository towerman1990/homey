package utils

import (
	"fmt"

	"github.com/bwmarrin/snowflake"
)

func GenUid() (uid int64, err error) {
	node, err := snowflake.NewNode(1)
	if err != nil {
		fmt.Println(err)
		return
	}

	return node.Generate().Int64(), err
}
