package database

import (
	"context"
	vContext "dev.hackerman.me/artheon/veverse-shared/context"
	"fmt"
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"os"
	"strconv"
	"time"
)

var Clickhouse driver.Conn

func SetupClickhouse(ctx context.Context) (context.Context, error) {
	clickhouseHost := os.Getenv("CLICKHOUSE_HOST")
	clickhousePort := os.Getenv("CLICKHOUSE_PORT")
	clickhouseUser := os.Getenv("CLICKHOUSE_USER")
	clickhousePass := os.Getenv("CLICKHOUSE_PASS")
	clickhouseName := os.Getenv("CLICKHOUSE_NAME")

	clickhousePortNum, err := strconv.Atoi(clickhousePort)
	if err != nil {
		return ctx, fmt.Errorf("failed to convert clickhouse port to int: %w", err)
	}

	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{fmt.Sprintf("%s:%d", clickhouseHost, clickhousePortNum)},
		Auth: clickhouse.Auth{
			Database: clickhouseName,
			Username: clickhouseUser,
			Password: clickhousePass,
		},
		Debug: true,
		Debugf: func(format string, v ...any) {
			fmt.Printf(format, v)
		},
		Settings: clickhouse.Settings{
			"max_execution_time": 60,
		},
		Compression: &clickhouse.Compression{
			Method: clickhouse.CompressionLZ4,
		},
		DialTimeout:      time.Duration(10) * time.Second,
		MaxOpenConns:     5,
		MaxIdleConns:     5,
		ConnMaxLifetime:  time.Duration(60) * time.Second,
		ConnOpenStrategy: clickhouse.ConnOpenInOrder,
		BlockBufferSize:  10,
	})
	if err != nil {
		return ctx, fmt.Errorf("failed to connect to clickhouse: %w", err)
	}

	Clickhouse = conn

	ctx = context.WithValue(ctx, vContext.Clickhouse, conn)

	return ctx, conn.Ping(context.Background())
}
