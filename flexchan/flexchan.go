package flexchan

import (
	"time"
	"sync/atomic"
	"reflect"
)

type FlexChan struct{
	statInterval  time.Duration
	userWorker    func(val FlexChanVal)
	statTicker    *time.Ticker
	chanId        int64
	mainChan      chan FlexChanVal
	workChan      chan FlexChanVal
	curCap        int
	userCh        chan FlexChanVal
	resetSelectCh chan struct{}
}

type FlexChanStat struct {
	Len, Cap int
	T	time.Time
}

type FlexChanVal struct {
	Stat *FlexChanStat
	Val  interface{}
}


func NewFlexChan(initialLen int) (fc *FlexChan) {
	fc = new(FlexChan)
	fc.mainChan = make(chan FlexChanVal, initialLen)
	fc.workChan = make(chan FlexChanVal, 100)
	fc.resetSelectCh = make(chan struct{}, 1)
	fc.curCap = initialLen
	go fc.worker()
	return
}

func (fc *FlexChan) Send(val interface{}) {
	fc.workChan <- FlexChanVal{Val:val}
}

func (fc *FlexChan) SetStatInterval(interval time.Duration) (){
	if interval == 0 {
		return
	}
	fc.statInterval = interval
	if fc.statTicker != nil {
		fc.statTicker.Stop()
	}
	fc.statTicker = time.NewTicker(fc.statInterval)
	fc.resetSelectCh <- struct{}{}
}

func (fc *FlexChan) AddScanChan(ch interface{}) (){
	rch := reflect.ValueOf(ch)
	if rch.Kind() != reflect.Chan {
		panic("not a chan")
	}
	if fc.userCh == nil {
		fc.userCh = make(chan FlexChanVal, 100)
		fc.resetSelectCh <- struct{}{}
	}
	go func() { //scan
		for {
			cc, val, ok := reflect.Select([]reflect.SelectCase{
				{Dir:reflect.SelectRecv,Chan:rch, Send:reflect.Value{}},
			});
			if !ok { //chan is closed
				return
			}
			if cc == 0 {
				fc.userCh <- FlexChanVal{Val:val.Interface()}
			}
		}
	}()
}
func (fc *FlexChan) SetReceiver(receiver func(fcv FlexChanVal, ok bool)) (){
	go func() {
		for {
			id := atomic.LoadInt64(&fc.chanId)
			select {
			case rv, ok := <-fc.mainChan:
				if !ok && atomic.LoadInt64(&fc.chanId) != id {
					continue //channel is closed by FlexChan.receiver
				}
				receiver(rv, ok)
			}
		}
	}()
}

func (fc *FlexChan) worker() {
	descendTicker := time.Tick(time.Minute * 15)

	if fc.statInterval != 0 {
		fc.statTicker = time.NewTicker(fc.statInterval)
	} else {
		fc.statTicker = &time.Ticker{}
	}
	updateChan := func() {
		newMainChan :=  make(chan FlexChanVal, fc.curCap)
		for copyfor:=true; copyfor;{
			select {
			case val := <- fc.mainChan: //get value
				newMainChan <- val
			default: //default
				copyfor=false
			}
		}
		oldMainChan := fc.mainChan
		fc.mainChan = newMainChan
		atomic.AddInt64(&fc.chanId,1)
		close(oldMainChan)
	}
	send2MainWithUpdate := func(val FlexChanVal, newcap int) {
		resend :
		select {
		case fc.mainChan <- val:
			//send ok
		default:
			fc.curCap = newcap
			updateChan()
			goto resend
		}
	}
	for {
		select {
		case <-fc.statTicker.C:
			send2MainWithUpdate(FlexChanVal{Stat:&FlexChanStat{Len: len(fc.mainChan),Cap:fc.curCap, T:time.Now()}}, fc.curCap*10)
		case <-descendTicker:
			curLen := len(fc.mainChan)
			if fc.curCap > 10 && curLen < fc.curCap/4 {
				fc.curCap /= 2
				updateChan()
			}
		case v:= <-fc.workChan:
			send2MainWithUpdate(v, fc.curCap*10)
		case v:= <-fc.userCh:
			send2MainWithUpdate(v, fc.curCap*10)
		case <-fc.resetSelectCh:
			//continue
		}
	}
}


