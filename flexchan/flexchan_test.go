package flexchan

import (
	"testing"
	"time"
	"sync/atomic"
	"sync"
	"sort"
)

var sendCount = int64(0)
var getCount = int64(0)
var sendChechsum = int64(0)
var getChechsum = int64(0)
type ctype struct {
	sender,val int
}

func TestFlexChan(t *testing.T) {

	fc := NewFlexChan(10)

	fc.SetStatInterval(time.Second)

	fc.SetReceiver(func(val FlexChanVal, ok bool) {
		if val.Stat == nil && val.Val != nil {
			cv := val.Val
			v := cv.(ctype)
			atomic.AddInt64(&getChechsum, int64(v.val))
			atomic.AddInt64(&getCount, 1)
		}
	})

	wg := sync.WaitGroup{}
	for i:=0; i< 1000; i++ {
		wg.Add(1)
		go func(int2 int) {
			sendWorker(fc,int2)
			wg.Done()
		}(i+1)
	}
	wg.Wait()

	time.Sleep(time.Second)

	if sendCount != getCount {
		t.Errorf("Send count not matched with received: %d != %d ", sendCount,getCount)
	}
	if sendChechsum != getChechsum {
		t.Errorf("Send count not matched with received: %d != %d ", sendChechsum,getChechsum)
	}
}

func sendWorker(fc *FlexChan,id int) {
	for i:= 0; i < 1000; i++ {
		v := id*1000+i
		atomic.AddInt64(&sendChechsum, int64(v))
		atomic.AddInt64(&sendCount, 1)
		fc.Send(ctype{id,v})
	}
	time.Sleep(2 * time.Second)
	for i:= 0; i < 1000; i++ {
		v := id*1000+i
		atomic.AddInt64(&sendChechsum, int64(v))
		atomic.AddInt64(&sendCount, 1)
		fc.Send(ctype{id,v})
	}
}

func TestFlexChanUserChan(t *testing.T) {

	userChan := make(chan chan []int, 1)
	fc := NewFlexChan(10)
	userMap := map[int]struct{}{}
	fc.SetReceiver(func(fcv FlexChanVal, ok bool) {
		switch fcv.Val.(type) {
		case int: //value from fc.Send
			val := fcv.Val.(int)
			userMap[val] = struct{}{}
		case chan []int: //value from userChan
			ch := fcv.Val.(chan []int)
			var list []int
			for i,_ := range userMap {
				list = append(list, i)
			}
			sort.Ints(list)
			ch <- list
		}
	})
	fc.AddScanChan(userChan)
	fc.Send(2)
	fc.Send(2)
	fc.Send(3)
	time.Sleep(time.Millisecond)
	requestChan := make(chan []int)
	userChan <- requestChan
	list := <-requestChan
	close(requestChan)
	if len(list) != 2 {
		t.Errorf("list len %d != 2 ",len(list))
	} else if list[0] != 2 || list[1] != 3 {
		t.Errorf("list content not matched")
	}

}