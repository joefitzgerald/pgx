package pgtype_test

import (
	"fmt"
	"io"
	"os"
	"reflect"
	"testing"

	"github.com/jackc/pgx"
	"github.com/jackc/pgx/pgtype"
)

func mustConnectPgx(t testing.TB) *pgx.Conn {
	config, err := pgx.ParseURI(os.Getenv("DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}

	conn, err := pgx.Connect(config)
	if err != nil {
		t.Fatal(err)
	}

	return conn
}

func mustClose(t testing.TB, conn interface {
	Close() error
}) {
	err := conn.Close()
	if err != nil {
		t.Fatal(err)
	}
}

type forceTextEncoder struct {
	e pgtype.TextEncoder
}

func (f forceTextEncoder) EncodeText(w io.Writer) error {
	return f.e.EncodeText(w)
}

type forceBinaryEncoder struct {
	e pgtype.BinaryEncoder
}

func (f forceBinaryEncoder) EncodeBinary(w io.Writer) error {
	return f.e.EncodeBinary(w)
}

func forceEncoder(e interface{}, formatCode int16) interface{} {
	switch formatCode {
	case pgx.TextFormatCode:
		return forceTextEncoder{e: e.(pgtype.TextEncoder)}
	case pgx.BinaryFormatCode:
		return forceBinaryEncoder{e: e.(pgtype.BinaryEncoder)}
	default:
		panic("bad encoder")
	}
}

func testSuccessfulTranscode(t testing.TB, pgTypeName string, values []interface{}) {
	conn := mustConnectPgx(t)
	defer mustClose(t, conn)

	ps, err := conn.Prepare("test", fmt.Sprintf("select $1::%s", pgTypeName))
	if err != nil {
		t.Fatal(err)
	}

	formats := []struct {
		name       string
		formatCode int16
	}{
		{name: "TextFormat", formatCode: pgx.TextFormatCode},
		{name: "BinaryFormat", formatCode: pgx.BinaryFormatCode},
	}

	for _, fc := range formats {
		ps.FieldDescriptions[0].FormatCode = fc.formatCode
		for i, v := range values {
			result := reflect.New(reflect.TypeOf(v))
			err := conn.QueryRow("test", forceEncoder(v, fc.formatCode)).Scan(result.Interface())
			if err != nil {
				t.Errorf("%v %d: %v", fc.name, i, err)
			}

			if !reflect.DeepEqual(result.Elem().Interface(), v) {
				t.Errorf("%v %d: expected %v, got %v", fc.name, i, v, result.Interface())
			}
		}
	}
}
