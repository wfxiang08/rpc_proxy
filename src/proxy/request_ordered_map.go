// This package provides a simple LRU cache. It is based on the
// LRU implementation in groupcache:
// https://github.com/golang/groupcache/tree/master/lru
package proxy

import (
	"container/list"
	"errors"
	log "github.com/wfxiang08/cyutils/utils/rolling_log"
	"sync"
)

//
// 特性:
// 1. 限定大小
// 2. 查看最后添加的元素(以便evict)
// 3. 元素按照添加的时间排序
//
// 代码设计原则:
// 大写开头的函数对外公开，负责加锁等；小写字母开头的函数，只负责完成逻辑，不考虑锁
//
type RequestMap struct {
	size      int
	evictList *list.List
	items     map[int32]*list.Element
	lock      sync.RWMutex
}

// 列表中的元素类型
type Entry struct {
	key   int32
	value *Request
}

func NewRequestMap(size int) (*RequestMap, error) {
	if size <= 0 {
		return nil, errors.New("Must provide a positive size")
	}
	c := &RequestMap{
		size:      size,
		evictList: list.New(),
		items:     make(map[int32]*list.Element, size),
	}
	return c, nil
}

// 返回所有元素，重新初始化List/Map
func (c *RequestMap) Purge() []*Request {
	c.lock.Lock()

	// 拷贝剩余的Requests
	results := make([]*Request, 0, len(c.items))
	for _, element := range c.items {
		results = append(results, element.Value.(*Entry).value)
	}

	// 重新初始化
	c.evictList = list.New()
	c.items = make(map[int32]*list.Element, c.size)

	c.lock.Unlock()

	return results
}

//
// 添加新的key, value
//
func (c *RequestMap) Add(key int32, value *Request) bool {
	c.lock.Lock()

	// 如果key存在，则覆盖之前的元素；并添加Warning
	if ent, ok := c.items[key]; ok {
		c.evictList.MoveToFront(ent)
		ent.Value.(*Entry).value = value
		log.Errorf(Red("Duplicated Key Found in RequestOrderedMap: %d"), key)

		c.lock.Unlock()
		return false
	}

	// Add new item
	ent := &Entry{key, value}
	entry := c.evictList.PushFront(ent)
	c.items[key] = entry

	// 如果超过指定的大小，则清除元素
	evict := c.evictList.Len() > c.size
	if evict {
		c.removeOldest()
	}
	c.lock.Unlock()
	return evict
}

//
// 读取Key, 并将它从Map中删除
//
func (c *RequestMap) Pop(key int32) *Request {
	c.lock.Lock()

	var result *Request = nil
	if ent, ok := c.items[key]; ok {
		c.removeElement(ent)
		result = ent.Value.(*Entry).value
	}

	c.lock.Unlock()
	return result
}

//
// 读取Key, 不调整元素的顺序
//
func (c *RequestMap) Get(key int32) (value *Request, ok bool) {
	c.lock.RLock()

	if ent, ok := c.items[key]; ok {
		value, ok = ent.Value.(*Entry).value, true
	}
	c.lock.RUnlock()
	return
}

func (c *RequestMap) Contains(key int32) (ok bool) {
	c.lock.RLock()
	_, ok = c.items[key]
	c.lock.RUnlock()
	return ok
}

//
// 删除指定的Key， 返回是否删除OK
//
func (c *RequestMap) Remove(key int32) bool {
	c.lock.Lock()

	result := false
	if ent, ok := c.items[key]; ok {
		c.removeElement(ent)
		result = true
	}
	c.lock.Unlock()
	return result
}

//
// RemoveOldest removes the oldest item from the cache.
//
func (c *RequestMap) RemoveOldest() {
	c.lock.Lock()
	c.removeOldest()
	c.lock.Unlock()
}

//
// 按照从旧到新的顺序返回 Keys的列表
//
func (c *RequestMap) Keys() []int32 {
	c.lock.RLock()

	keys := make([]int32, len(c.items))
	ent := c.evictList.Back()
	i := 0
	for ent != nil {
		keys[i] = ent.Value.(*Entry).key
		ent = ent.Prev()
		i++
	}
	c.lock.RUnlock()
	return keys
}

//
// 获取当前的元素个数
//
func (c *RequestMap) Len() int {
	c.lock.RLock()
	result := c.evictList.Len()
	c.lock.RUnlock()
	return result
}

//
// 清除过期的Request
//
func (c *RequestMap) RemoveExpired(expiredInMicro int64) {
	c.lock.Lock()

	for true {
		ent := c.evictList.Back()

		// 如果Map为空，则返回
		if ent == nil {
			break
		}

		// 如果请求还没有过期，则不再返回
		entry := ent.Value.(*Entry)
		request := entry.value
		if request.Start > expiredInMicro {
			break
		}

		// 1. 准备删除当前的元素
		c.removeElement(ent)

		// 2. 如果出问题了，则打印原始的请求的数据
		log.Warnf(Red("Remove Expired Request: %s.%s [%d]"),
			request.Service, request.Request.Name, request.Response.SeqId)

		// 3. 处理Request
		request.Response.Err = request.NewTimeoutError()
		request.Wait.Done()
	}

	c.lock.Unlock()
}

//
// 读取最旧的元素
//
func (c *RequestMap) PeekOldest() (key int32, value *Request, ok bool) {
	c.lock.RLock()
	ent := c.evictList.Back()

	if ent != nil {
		entry := ent.Value.(*Entry)
		key, value, ok = entry.key, entry.value, true
	} else {
		key, value, ok = 0, nil, false
	}

	c.lock.RUnlock()
	return
}

// 删除最旧的元素
func (c *RequestMap) removeOldest() {
	ent := c.evictList.Back()
	if ent != nil {
		c.removeElement(ent)
	}
}

// 删除指定的元素(参数: list.Element)
func (c *RequestMap) removeElement(e *list.Element) {

	c.evictList.Remove(e)
	kv := e.Value.(*Entry)

	//	log.Printf("Remove Element: %s, With key: %d", kv.value.Request.Name, kv.key)
	delete(c.items, kv.key)
}
