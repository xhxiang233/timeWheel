package utility

import (
	"container/list"
	"log"
	"sync"
	"time"
)

/*
   用map来存放每一个链表，有多少numTick就有多少链表，每一个链表里边都能存放任意多个任务，每到下一个链表就进行检测，看是否有需要执行的任务，有的话就起一个go程去执行，没有就正常走下一个链表
   每次走一个链表，都要判断是否到底，到底的话要从头开始
*/

//type pos struct {
//	key int
//	e   *list.Element //*list.Element存放了元素的位置信息，以及和其他元素的关联信息
//}
var lock sync.Mutex

// Task 用于接收添加的任务
type Task struct {
	do         func(interface{})
	id         string
	isCronTask int
	time       int
}

type TimerWheel struct {
	numTick int //每一轮的滴答数
	perTick int //每次滴答的时间间隔
	//do         func(interface{})
	ticker *time.Ticker //一个自动收报机持有一个通道，它每隔一段时间就发送一个时钟的“滴答声”。
	//index      map[interface{}]pos
	pointer    int //list的指针，指向当前的元素
	wheel      map[int]*list.List
	stopSignal chan int
	taskList   *list.List
}

// NewTimeWheel 请确保每一轮的滴答数大于最大的周期值
func NewTimeWheel(numTick int, perTick int) *TimerWheel {
	if numTick <= 0 || perTick <= 0 {
		return nil
	}
	tw := &TimerWheel{
		numTick: numTick,
		perTick: perTick,
		ticker:  time.NewTicker(time.Duration(perTick) * time.Second), //time.NewTicker返回一个包含通道的新计时器
		pointer: 0,
		wheel:   make(map[int]*list.List),
		//index:      make(map[interface{}]pos),
		stopSignal: make(chan int),
		//taskList:   list.New(),
	}
	for i := 0; i <= numTick-1; i++ { //产生了numTick个（每一轮的滴答数）初始化的链表
		l := list.New() //返回一个初始化的列表
		tw.wheel[i] = l
	}
	return tw
}

func (tw *TimerWheel) Start() {
	for {
		select {
		case <-tw.ticker.C:
			p := tw.pointer
			//log.Println("现在是第", p, "个链表")
			if e := tw.wheel[p].Front(); e != nil {
				//log.Println(e.Value)
				t := (e.Value).(Task)
				tw.Do(p, t, e)
				eNext := e.Next()
				for { //判断该槽是否还有任务
					if eNext != nil {
						t = (eNext.Value).(Task)
						tw.Do(p, t, eNext)
					} else {
						break
					}
					eNext = eNext.Next()
				}
			}
			if tw.pointer >= tw.numTick-1 {
				tw.pointer = 0
			} else {
				tw.pointer++
			}

		case signal := <-tw.stopSignal: //停止信号
			if signal == 1 {
				tw.Stop()
			}
		}
	}
}

func (tw *TimerWheel) Stop() {
	tw.ticker.Stop()
}

// Add 在时间轮里添加函数
//isCronTask如果为0，则不是循环任务，如果为1，则是循环任务
//time为任务延迟执行时间
//task为要执行的任务
func (tw *TimerWheel) Add(isCronTask int, time int, task func(interface{}), id string) {
	go func() {
		lock.Lock()
		defer lock.Unlock()
		p := tw.pointer
		var l *list.List
		if p+time >= len(tw.wheel) {
			l = tw.wheel[p+time-len(tw.wheel)]
		} else {
			l = tw.wheel[p+time]
		}
		t := Task{
			do:         task,
			id:         id,
			isCronTask: isCronTask,
			time:       time,
		}
		l.PushBack(t)
	}()
}

// Do 执行函数
func (tw *TimerWheel) Do(p int, task Task, e *list.Element) {
	go func() {
		lock.Lock()
		defer lock.Unlock()
		l := tw.wheel[p]
		data := e.Value
		go task.do(data)
		if task.isCronTask != 0 { //判断当前任务是否是循环任务,如果是循环任务，则删除当前的任务点，寻找下次触发的任务点
			cronTime := task.time
			if len(tw.wheel)-p-1 < cronTime {
				lNew := tw.wheel[cronTime-(len(tw.wheel)-p)] //移植循环的任务点
				lNew.PushBack(data)
			} else {
				lNew := tw.wheel[p+cronTime]
				lNew.PushBack(data)
			}
		}
		l.Remove(e)
	}()
}

// Remove 移除时间轮里的函数
func (tw *TimerWheel) Remove(id string) {
	go func() {
		lock.Lock()
		defer lock.Unlock()
		log.Println("时间轮开始 " + id + " 的移除检查")
		for _, l := range tw.wheel {
			if e := l.Front(); e != nil {
				t := (e.Value).(Task)
				if t.id == id {
					l.Remove(e)
				} else if e.Next() != nil {
					eNext := e.Next()
					for { //判断该槽是否有该函数
						if eNext != nil {
							t := (eNext.Value).(Task)
							if t.id == id {
								l.Remove(e)
								log.Println(id + "已移除！")
							}
						} else {
							break
						}
						eNext = eNext.Next()
					}
				}
			}
		}
	}()
}
