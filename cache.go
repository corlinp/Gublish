package main

import "net/http"

const historyLength = 100

//const cacheTimeout = 1 * time.Second

// This will store the last X values of the stream
type Cache struct {
	head         *CacheTreeNode
	tail         *CacheTreeNode
	size         int
	subscription Subscription
}

// I'm treating the list of caches as a linked list.
// This should allow for safe concurrent access without locking
type CacheTreeNode struct {
	next *CacheTreeNode
	data []byte
}

func NewCache(stream string, broker *Broker) (cache *Cache) {
	cache = &Cache{
		subscription: Subscription{
			channel: make(chan []byte),
			stream:  stream,
		},
	}
	return
}

func (cache *Cache) add(data []byte) {
	if cache.head == nil {
		cache.head = &CacheTreeNode{
			data: data,
		}
		cache.tail = cache.head
	} else {
		newNode := &CacheTreeNode{
			data: data,
		}
		cache.tail.next = newNode
		cache.tail = newNode
	}

	cache.size++
	if cache.size > historyLength {
		cache.head = cache.head.next
		cache.size--
	}
}

func (cache *Cache) get(rw http.ResponseWriter) {
	curNode := cache.head
	for curNode != nil {
		rw.Write(ssePrefix)
		rw.Write(curNode.data)
		rw.Write(sseSuffix)
		curNode = curNode.next
	}
}

func (cache *Cache) listen(broker *Broker) {
	broker.newClients <- cache.subscription
	defer func() {
		broker.closingClients <- cache.subscription
	}()

	for {
		select {
		default:
			cache.add(<-cache.subscription.channel)
		}
	}
}
