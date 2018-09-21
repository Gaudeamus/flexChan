package flexchan

import (
	"reflect"
	"time"
	"sync/atomic"
)

type FlexChan struct{
	userWorkerInterval time.Duration
	userWorker func(FlexChanStat)
	userTicker *time.Ticker
	chanId   int64
	mainChan reflect.Value
	workChan chan interface{}
	curCap   int
}

type FlexChanStat struct {
	Len, Cap int
}


func NewFlexChan(chanType interface{}, initialLen int) (fc *FlexChan) {
	v := reflect.ValueOf(chanType)
	vct := reflect.ChanOf(reflect.BothDir,v.Type())
	fc = new(FlexChan)
	fc.mainChan = reflect.MakeChan(vct,initialLen)
	fc.workChan = make(chan interface{}, 100)
	fc.curCap = initialLen
	go fc.worker()
	return
}

func (fc *FlexChan) Send(val interface{}) {
	fc.workChan <- val
}

func (fc *FlexChan) SetIntervalWorker(worker func(FlexChanStat), interval time.Duration) (){
	if interval == 0 || worker == nil {
		return
	}
	fc.userWorkerInterval = interval
	fc.userWorker = worker
	if fc.userTicker != nil {
		fc.userTicker.Stop()
	}
	fc.userTicker = time.NewTicker(fc.userWorkerInterval)
}

func (fc *FlexChan) SetReceiver(receiver func(val interface{}, ok bool)) (){

	go func() {
		for {
			sel := []reflect.SelectCase{{Dir:reflect.SelectRecv,Chan:fc.mainChan, Send:reflect.Value{}}}
			id := atomic.LoadInt64(&fc.chanId)
			_,rv,ok := reflect.Select(sel)
			if !ok && atomic.LoadInt64(&fc.chanId) != id {
				continue //channel is closed by flexchan.receiver
			}
			receiver(rv.Interface(), ok)
		}
	}()
}

func (fc *FlexChan) worker() {
	tic := time.Tick(time.Minute * 15)

	if fc.userWorkerInterval != 0 {
		fc.userTicker = time.NewTicker(fc.userWorkerInterval)
	}
	updateChan := func() {
		newMainChan := reflect.MakeChan(fc.mainChan.Type(), fc.curCap)
		cases := []reflect.SelectCase{
			{Dir:reflect.SelectRecv,Chan:fc.mainChan, Send:reflect.Value{}},
			{Dir:reflect.SelectDefault,Chan:reflect.Value{}, Send:reflect.Value{}},
		}
		for copyfor:=true; copyfor;{
			cc,val,_ := reflect.Select(cases)
			switch cc {
			case 0: //get value
				newMainChan.Send(val)
			case 1: //default
				copyfor=false
			}
		}
		oldMainChan := fc.mainChan
		fc.mainChan = newMainChan
		atomic.AddInt64(&fc.chanId,1)
		oldMainChan.Close()
	}
	for {
		select {
		case <-fc.userTicker.C:
			fc.userWorker(FlexChanStat{fc.mainChan.Len(),fc.curCap})
		case <-tic:
			curLen := fc.mainChan.Len()
			if fc.curCap > 10 && curLen < fc.curCap/4 {
				fc.curCap /= 2
				updateChan()
			}
		case v:= <-fc.workChan:
		resend:
			cases := []reflect.SelectCase{
				{Dir:reflect.SelectSend,Chan:fc.mainChan, Send:reflect.ValueOf(v)},
				{Dir:reflect.SelectDefault,Chan:reflect.Value{}, Send:reflect.Value{}},
			}
			cc,_,_ := reflect.Select(cases)
			switch cc {
			case 0:
				//send ok
			case 1:
				fc.curCap *= 10
				updateChan()
				goto resend
			}
		}
	}
}

