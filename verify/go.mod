module github.com/Xugu-Open-Source/xugu-xorm/verify

go 1.13

require (
	gitee.com/XuguDB/go-xugu-driver v1.0.13
	github.com/Xugu-Open-Source/xugu-xorm v0.0.0
	xorm.io/xorm v1.3.2
)

// 本仓库内 module 引用：clone 后天然生效，指向同仓库的父目录
replace github.com/Xugu-Open-Source/xugu-xorm => ../
