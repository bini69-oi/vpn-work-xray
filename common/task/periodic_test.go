package task_test

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/xtls/xray-core/common"
	. "github.com/xtls/xray-core/common/task"
)

func TestPeriodicTaskStop(t *testing.T) {
	var value atomic.Int64
	task := &Periodic{
		Interval: time.Second * 2,
		Execute: func() error {
			value.Add(1)
			return nil
		},
	}
	common.Must(task.Start())
	time.Sleep(time.Second * 5)
	common.Must(task.Close())
	current := value.Load()
	if current != 3 {
		t.Fatal("expected 3, but got ", current)
	}
	time.Sleep(time.Second * 4)
	current = value.Load()
	if current != 3 {
		t.Fatal("expected 3, but got ", current)
	}
	common.Must(task.Start())
	time.Sleep(time.Second * 3)
	current = value.Load()
	if current != 5 {
		t.Fatal("Expected 5, but ", current)
	}
	common.Must(task.Close())
}
