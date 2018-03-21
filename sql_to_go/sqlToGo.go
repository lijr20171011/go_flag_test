package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"

	"github.com/urfave/cli" // go get github.com/urfave/cli

	"os"
	"path"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

type DBTableInfo struct {
	ColumnName    string `json:"column_name"`    // 字段名
	DataType      string `json:"data_type"`      // 数据类型
	ColumnComment string `json:"column_comment"` // 备注
	ColumnKey     string `json:"column_key"`     // 索引
}

func main() {
	sqltogo()
}

// flag包
func sqltogo1() {
	var dbName, tableName, host, user, pwd string
	var writeFile bool
	{ // 库名
		flag.StringVar(&dbName, "db", "ljr", "数据库库名")
		flag.StringVar(&dbName, "d", "ljr", "数据库库名(short)")
	}
	{ // 表名
		flag.StringVar(&tableName, "table", "users", "表名")
		flag.StringVar(&tableName, "t", "users", "表名(short)")
	}
	{ // 主机地址
		flag.StringVar(&host, "host", "127.0.0.1", "数据库主机地址")
		flag.StringVar(&host, "h", "127.0.0.1", "数据库主机地址(short)")
	}
	{ // 用户名
		flag.StringVar(&user, "user", "root", "用户名")
		flag.StringVar(&user, "u", "root", "用户名(short)")
	}
	{ // 密码
		flag.StringVar(&pwd, "pwd", "123456", "密码")
		flag.StringVar(&pwd, "p", "123456", "密码(short)")
	}
	{ // 是否写入文件
		flag.BoolVar(&writeFile, "writefile", false, "是否写入文件")
		flag.BoolVar(&writeFile, "w", false, "是否写入文件(short)")

	}
	flag.Parse()
	GetDBTableStruct(host, user, pwd, dbName, tableName, writeFile)
	return
}

// cli包
func sqltogo() {
	app := cli.NewApp()
	app.Name = "sqltogo"
	app.Usage = "用途:根据库名表名将表结构转化为go结构体"
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "db",
			Value: "ljr",
			Usage: "数据库库名",
		},
		cli.StringFlag{
			Name:  "table",
			Value: "users",
			Usage: "表名",
		},
		cli.StringFlag{
			Name:  "host",
			Value: "127.0.0.1",
			Usage: "数据库主机地址",
		},
		cli.StringFlag{
			Name:  "user",
			Value: "root",
			Usage: "用户名",
		},
		cli.StringFlag{
			Name:  "pwd",
			Value: "123456",
			Usage: "密码",
		},
		cli.BoolFlag{
			Name:  "writefile,w",
			Usage: "是否写入文件",
		},
	}
	app.Action = func(c *cli.Context) error {
		dbName := c.String("db")
		tableName := c.String("table")
		host := c.String("host")
		user := c.String("user")
		pwd := c.String("pwd")
		var writeFile bool
		// 判断写入功能是否已设置
		isWriteSet := c.IsSet("writefile")
		Info(isWriteSet)
		if isWriteSet {
			writeFile = c.Bool("writefile")
		} else {
			isWrite := ""
			fmt.Print("\n需要写入文件吗?(y/n)\n> ")
		LOOP:
			for {
				fmt.Scan(&isWrite)
				switch isWrite {
				case "yes", "y", "YES", "Y", "Yes":
					writeFile = true
					break LOOP
				case "no", "n", "NO", "N", "No":
					writeFile = false
					break LOOP
				default:
					fmt.Print("\n输入异常,请重新输入！需要写入文件吗?(y/n)\n> ")
				}
			}
		}
		GetDBTableStruct(host, user, pwd, dbName, tableName, writeFile)
		return nil
	}
	err := app.Run(os.Args)
	if IsErr(err) {
		return
	}
}

func GetDBTableStruct(host, user, pwd, dbName, tableName string, writeFile bool) {
	// 连接数据库
	db, err := sql.Open("mysql", user+":"+pwd+"@tcp("+host+":3306)/"+dbName+"?charset=utf8")
	if IsErr(err) {
		return
	}
	// 查询
	sql := `SELECT column_name,data_type,column_comment,column_key FROM information_schema.columns WHERE table_schema = ? AND table_name = ? `
	rows, err := db.Query(sql, dbName, tableName)
	if IsErr(err) {
		return
	}
	// 解析参数
	infos := rowsToStruct(rows)
	// 整理数据
	hasTime := false
	structStr := "\n\ntype " + UnderlineToUperCase(tableName) + " struct {\n"
	for _, v := range infos {
		// 字段名
		structStr += "\t" + UnderlineToUperCase(v.ColumnName) + "\t"
		// 字段类型
		switch v.DataType {
		case "int", "tinyint":
			structStr += "int" + "\t"
		case "float", "double", "decimal":
			structStr += "float64" + "\t"
		case "date", "datetime", "time", "timestamp":
			structStr += "time.Time" + "\t"
			hasTime = true
		case "char", "varchar", "text", "longtext":
			structStr += "string" + "\t"
		case "bigint":
			structStr += "int64" + "\t"
		default:
			IsErr(errors.New("数据类型不明 --> " + v.DataType))
		}
		// 字段说明
		structStr += "`orm:"
		structStr += `"column(` + v.ColumnName + `)`
		if v.ColumnKey == "PRI" {
			structStr += ";pk"
		}
		structStr += `"` + "`\t"
		// 注释
		if v.ColumnComment != "" {
			structStr += "// " + v.ColumnComment
		}
		structStr += "\n"
	}
	structStr += "}\n"
	if writeFile {
		// 写入文件
		headInfo := "package " + tableName + "\n"
		if hasTime {
			headInfo += "\nimport(\n\t" + `"time"` + "\n)\n"
		}
		structToFile(tableName, headInfo+structStr)
	} else {
		Info(structStr)
	}
	return
}

func structToFile(tableName, fileInfo string) {
	path := "/"
	// 获取当前系统分隔符
	if os.IsPathSeparator('\\') {
		path = "\\"
	}
	// 获取当前目录
	dir, err := os.Getwd()
	if IsErr(err) {
		return
	}
	// 判断models是否已存在
	modelsPaht := dir + path + "models"
	if !IsExit(modelsPaht) {
		// 不存在创建models目录
		err = os.Mkdir(modelsPaht, os.ModePerm)
		if IsErr(err) {
			return
		}
	}
	// 判断 table文件是否已存在
	filePath := modelsPaht + path + tableName + ".go"
	// var f *os.File
	// if IsExit(filePath) {
	// 	// 文件已存在直接打开
	// 	f, err = os.OpenFile(filePath, os.O_APPEND, 0666)
	// 	if IsErr(err) {
	// 		return
	// 	}
	// 	defer f.Close()
	// } else {
	// 	// 不存在创建tableName文件
	// 	f, err = os.Create(filePath)
	// 	if IsErr(err) {
	// 		return
	// 	}
	// 	defer f.Close()
	// 	_, err = f.WriteString("package models\n")
	// 	if IsErr(err) {
	// 		return
	// 	}
	// }
	// // 将结构体写入文件
	// _, err = f.WriteString(structInfo)
	// if IsErr(err) {
	// 	return
	// }
	if IsExit(filePath) {
		IsErr(errors.New("models目录下已有" + tableName + "文件"))
		return
	}
	// 创建tableName文件
	f, err := os.Create(filePath)
	if IsErr(err) {
		return
	}
	defer f.Close()
	_, err = f.WriteString(fileInfo)
	if IsErr(err) {
		return
	}
	return
}

// 获取查询结果
func rowsToStruct(rows *sql.Rows) (infos []DBTableInfo) {
	// 获取目标结构体
	columns, err := rows.Columns()
	if IsErr(err) {
		return
	}
	l := len(columns)
	for rows.Next() {
		var c DBTableInfo
		values := make([]interface{}, l)
		p := make([]interface{}, l)
		m := map[string]string{}
		for i, _ := range columns {
			p[i] = &values[i]
		}
		err = rows.Scan(p...)
		for i, v := range columns {
			m[v] = string(values[i].([]byte))
		}
		data, err := json.Marshal(m)
		if IsErr(err) {
			return
		}
		err = json.Unmarshal(data, &c)
		if IsErr(err) {
			return
		}
		infos = append(infos, c)
	}
	return
}

// 判断文件/文件夹是否存在
func IsExit(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

// 删除下划线,首字母大写
func UnderlineToUperCase(str string) string {
	strs := strings.Split(str, "_")
	s := ""
	for _, v := range strs {
		r := []rune(v)
		if r[0] >= 97 && r[0] <= 122 {
			r[0] = r[0] - 32
		}
		s += string(r)
	}
	return s
}

// 打印参数
func Info(v ...interface{}) {
	format := strings.Repeat("%v ", len(v))
	msg := fmt.Sprintf(format, v...)
	_info(2, msg)
}

// 检查错误,如果err不为空则打印返回true
func IsErr(err error) bool {
	if err != nil {
		msg := fmt.Sprintf("错误信息: %v ", err)
		_info(2, msg)
		return true
	}
	return false
}

// 检查错误,如果err不为空则打印并退出
func ExitWithErr(err error) bool {
	if err != nil {
		msg := fmt.Sprintf("错误信息: %v ", err)
		_info(2, msg)
		syscall.Exit(557)
		return true
	}
	return false
}

func _info(step int, msg string) {
	now := time.Now().Format("2006-01-02 15:04:05")
	_, filePath, line, ok := runtime.Caller(step)
	if !ok {
		filePath = "????"
		line = 0
	}
	//获取文件名
	_, file := path.Split(filePath)
	fmt.Println(now + " " + "[" + file + ":" + strconv.Itoa(line) + "]" + msg)
}
