package structure

import "math/rand"

type skipListNode struct {
	next   []*skipListNode
	height int
	key    string
	value  any
}

func newSkipListNode(key string, value any, height int) *skipListNode {
	return &skipListNode{
		next:   make([]*skipListNode, height),
		height: height,
		key:    key,
		value:  value,
	}
}

func (node *skipListNode) getNextNode(level int) *skipListNode {
	return node.next[level]
}

func (node *skipListNode) changeNextNode(level int, next *skipListNode) {
	old := node.next[level]
	node.next[level] = next
	next.next[level] = old
}

func (node *skipListNode) removeNextNode(level int) {
	new_ := node.next[level].next[level]
	node.next[level] = new_
}

type SkipList struct {
	size  int
	level int
	head  *skipListNode
}

func NewSkipList(level int) *SkipList {
	return &SkipList{
		size:  0,
		level: level,
		head:  newSkipListNode("", "", level),
	}
}

func randomHeight(max int) int {
	level := 1
	for rand.Int()%2 != 0 {
		level++
	}
	if level > max {
		return max
	}
	return level
}

func (sl *SkipList) Insert(key string, value any) {

	// 需要找到每一个层次的前驱
	prevs := make([]*skipListNode, sl.level)
	cur := sl.head

	// 每一个 prev 的 key 小于等于需要插入的 key
	for i := sl.level - 1; i >= 0; i-- {
		for nxt := cur.getNextNode(i); nxt != nil && nxt.key <= key; nxt = cur.getNextNode(i) {
			cur = nxt
		}
		prevs[i] = cur
	}

	if prevs[0].key == key {
		prevs[0].value = value
		return
	}
	// 随机生成高度节点
	height := randomHeight(sl.level)
	node := newSkipListNode(key, value, height)

	// 从底层到高层依次插入
	for i := 0; i < height; i++ {
		prevs[i].changeNextNode(i, node)
	}
	sl.size++

}

func (sl *SkipList) InsertIfNotExist(key string, value any) bool {

	// 需要找到每一个层次的前驱
	prevs := make([]*skipListNode, sl.level)
	cur := sl.head
	// 每一个 prev 的 key 小于等于需要插入的 key
	for i := sl.level - 1; i >= 0; i-- {
		for nxt := cur.getNextNode(i); nxt != nil && nxt.key <= key; nxt = cur.getNextNode(i) {
			cur = nxt
		}
		prevs[i] = cur
	}

	// 如果前驱 key 相同则判断插入失败
	if prevs[0].key == key {
		return false
	}

	// 随机生成高度节点
	height := randomHeight(sl.level)
	node := newSkipListNode(key, value, height)

	// 从底层到高层依次插入
	for i := 0; i < height; i++ {
		prevs[i].changeNextNode(i, node)
	}
	sl.size++
	return true
}

func (sl *SkipList) Update(key string, value any) bool {
	// 需要找到每一个层次的前驱
	cur := sl.head

	// 每一个 prev 的 key 小于等于需要插入的 key
	for i := sl.level - 1; i >= 0 && cur.key < key; i-- {
		for nxt := cur.getNextNode(i); nxt != nil && nxt.key <= key; nxt = cur.getNextNode(i) {
			cur = nxt
		}
	}

	if cur.key == key {
		cur.value = value
		return true
	}
	return false
}

func (sl *SkipList) Get(key string) (any, bool) {
	// 需要找到每一个层次的前驱
	cur := sl.head

	// 每一个 prev 的 key 小于等于需要插入的 key
	for i := sl.level - 1; i >= 0 && cur.key < key; i-- {
		for nxt := cur.getNextNode(i); nxt != nil && nxt.key <= key; nxt = cur.getNextNode(i) {
			cur = nxt
		}
	}

	if cur.key == key {
		return cur.value, true
	}
	return nil, false
}

func (sl *SkipList) Delete(key string) {

	// 需要找到每一个层次的前驱
	prevs := make([]*skipListNode, sl.level)
	cur := sl.head

	// 每一个 prev 的 key 小于需要插入的 key，然后判断下一个键
	for i := sl.level - 1; i >= 0; i-- {
		for nxt := cur.getNextNode(i); nxt != nil && nxt.key < key; nxt = cur.getNextNode(i) {
			cur = nxt
		}
		prevs[i] = cur
	}

	height := 0
	if prevs[0].getNextNode(0).key == key {
		height = prevs[0].getNextNode(0).height
	}

	// 从底层到高层依次插入
	for i := 0; i < height; i++ {
		prevs[i].removeNextNode(i)
	}
	sl.size--

}

func (sl *SkipList) Exist(key string) bool {
	// 需要找到每一个层次的前驱
	cur := sl.head

	// 每一个 prev 的 key 小于等于需要插入的 key
	for i := sl.level - 1; i >= 0 && cur.key < key; i-- {
		for nxt := cur.getNextNode(i); nxt != nil && nxt.key <= key; nxt = cur.getNextNode(i) {
			cur = nxt
		}
	}

	return cur.key == key
}

func (sl *SkipList) Size() int {
	return sl.size
}
