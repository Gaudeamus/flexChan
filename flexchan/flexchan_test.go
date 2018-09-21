package flexchan

import (
	"testing"
	"time"
	"sync/atomic"
	"sync"
)

var sendCount = int64(0)
var getCount = int64(0)
var sendChechsum = int64(0)
var getChechsum = int64(0)
type ctype struct {
	sender,val int
}

func TestFlexChan(t *testing.T) {

	fc := NewFlexChan(ctype{},10)

	fc.SetIntervalWorker(func(stat FlexChanStat) {},time.Second)

	fc.SetReceiver(func(val interface{}, ok bool) {
		v := val.(ctype)
		atomic.AddInt64(&getChechsum, int64(v.val))
		atomic.AddInt64(&getCount, 1)
	})

	wg := sync.WaitGroup{}
	for i:=0; i< 1000; i++ {
		wg.Add(1)
		go func(int2 int) {
			sendWorker3(fc,int2)
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

func sendWorker3(fc *FlexChan,id int) {
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