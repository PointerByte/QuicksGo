module github.com/PointerByte/QuicksGo

go 1.26

replace (
	github.com/PointerByte/QuicksGo/config => ./logger
	github.com/PointerByte/QuicksGo/logger => ./logger
	github.com/PointerByte/QuicksGo/security => ./security
)
