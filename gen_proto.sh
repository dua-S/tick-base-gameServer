#!/bin/bash

# 进入 proto 目录
cd proto || exit

# 执行生成命令
# 注意：*.proto 表示编译该目录下所有的 proto 文件
protoc --go_out=. --go_opt=paths=source_relative \
       --go-grpc_out=. --go-grpc_opt=paths=source_relative \
       *.proto

echo "Proto code generated successfully."