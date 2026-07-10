package main

import (
	"log"

	"github.com/yang/wormhole_backend/internal/config"
	"github.com/yang/wormhole_backend/internal/db"
	"gorm.io/gen"
)

// gorm/gen 代码生成器。
//
// 用法：先在 .env 里配好数据库连接，并确保表已存在（可执行 migrations/ 下的 SQL），
// 然后运行：
//
//	go run ./cmd/gen
//
// 生成物会写入 internal/dal/model 与 internal/dal/query，均带 "DO NOT EDIT" 头，
// 请勿手改。生成后可删除手写的 internal/dal/model/users.go 占位文件。
func main() {
	cfg := config.LoadConfig()
	database := db.ConnectDB(cfg)

	g := gen.NewGenerator(gen.Config{
		OutPath:      "internal/dal/query",
		ModelPkgPath: "internal/dal/model",
		// 生成字段的指针类型以区分零值与 NULL，并生成默认查询方法。
		Mode:              gen.WithoutContext | gen.WithDefaultQuery | gen.WithQueryInterface,
		FieldNullable:     true,
		FieldCoverable:    true,
		FieldSignable:     true,
		FieldWithIndexTag: true,
		FieldWithTypeTag:  true,
	})

	g.UseDB(database)

	// 为库中所有表生成 model + query。
	g.ApplyBasic(g.GenerateAllTable()...)

	g.Execute()
	log.Println("DAL 代码生成完成：internal/dal/model, internal/dal/query")
}
