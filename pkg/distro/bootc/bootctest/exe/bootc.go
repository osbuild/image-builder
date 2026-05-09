package main

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"time"
)

func fakeBootc() error {
	if len(os.Args) >= 3 && os.Args[1] == "install" && os.Args[2] == "print-configuration" {
		fmt.Println(`{"filesystem": {"root": {"type": "ext4"}}}`)
		return nil
	}
	if len(os.Args) >= 4 && os.Args[1] == "container" && os.Args[2] == "inspect" && os.Args[3] == "--json" {
		fmt.Println(`{"kernel": {"unified": false}}`)
		return nil
	}
	return fmt.Errorf("unexpected bootc arguments %v", os.Args)
}

func fakeSleep() error {
	if os.Args[1] != "infinity" {
		return fmt.Errorf("unexpected sleep arguments %v", os.Args)
	}
	time.Sleep(math.MaxInt64)
	return nil
}

func main() {
	var err error
	switch filepath.Base(os.Args[0]) {
	case "bootc":
		err = fakeBootc()
	case "sleep":
		err = fakeSleep()
	}
	if err != nil {
		println("error: ", err.Error())
		os.Exit(1)
	}
}
