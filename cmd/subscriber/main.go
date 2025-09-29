package main

import "github.com/architeacher/svc-web-analyzer/internal/runtime"

func main() {
	runtime.NewSubscriber().Run()
}