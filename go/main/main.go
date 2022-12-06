package main

import "sync"

type C struct {
	lock sync.Mutex
}

func c() sync.Mutex {
	lock := sync.Mutex{}
	lock.Lock()
	return lock
}

func main() {

}
