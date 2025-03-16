package mr

import (
	"errors"
	"log"
	"sync"
	"sync/atomic"
	"time"
)
import "net"
import "os"
import "net/rpc"
import "net/http"

const TimeOut = 10 * time.Second

type Phase int

const (
	ReadyForMapping  Phase = 1
	ReadyForReducing Phase = 2
	AllTasksComplete Phase = 3
)

type Coordinator struct {
	// Your definitions here.
	files        []string
	nReduce      int
	phase        Phase
	taskQueue    chan Task
	taskProgress map[int]chan struct{}
	taskId       atomic.Int64
	wg           sync.WaitGroup
}

// Your code here -- RPC handlers for the worker to call.

// an example RPC handler.
//
// the RPC argument and reply types are defined in rpc.go.
func (c *Coordinator) Example(args *ExampleArgs, reply *ExampleReply) error {
	reply.Y = args.X + 1
	return nil
}

// AssignTask 为 worker 根据 taskQueue 分配 map 或 reduce 任务
func (c *Coordinator) AssignTask(args *AssignTaskArgs, reply *AssignTaskReply) error {
	if c.phase == ReadyForMapping || c.phase == ReadyForReducing {
		task := <-c.taskQueue
		reply.Task = task
		// 开启一个 goroutine 若 10s 内未返回完成进度则重新分配任务，若收到则当作任务执行完毕
		taskId := task.TaskId
		c.taskProgress[taskId] = make(chan struct{})
		go func(taskProgress chan struct{}) {
			timer := time.NewTimer(TimeOut)
			defer timer.Stop()
			select {
			case <-taskProgress:
				return
			case <-timer.C:
				c.taskQueue <- task
			}
		}(c.taskProgress[taskId])
	} else {
		c.Done()
	}
	return nil
}

// TaskComplete worker 任务执行完毕后告知 coordinator，后者记录任务完成情况
func (c *Coordinator) TaskComplete(args *TaskCompleteArgs, reply *TaskCompleteReply) error {
	taskId := args.TaskId
	taskProgress := c.taskProgress[taskId]
	if taskProgress == nil {
		return errors.New("task Not Found")
	}
	taskProgress <- struct{}{}
	c.wg.Done()
	return nil
}

// start a thread that listens for RPCs from worker.go
func (c *Coordinator) server() {
	rpc.Register(c)
	rpc.HandleHTTP()
	// l, e := net.Listen("tcp", ":1234")
	sockname := coordinatorSock()
	os.Remove(sockname)
	l, e := net.Listen("unix", sockname)
	if e != nil {
		log.Fatal("listen error:", e)
	}
	go http.Serve(l, nil)
}

// main/mrcoordinator.go calls Done() periodically to find out
// if the entire job has finished.
func (c *Coordinator) Done() bool {
	return c.phase == AllTasksComplete
}

// create a Coordinator.
// main/mrcoordinator.go calls this function.
// nReduce is the number of reduce tasks to use.
func MakeCoordinator(files []string, nReduce int) *Coordinator {
	// 1. 初始化构造 coordinator 结构体
	c := Coordinator{
		files:        files,
		nReduce:      nReduce,
		phase:        ReadyForMapping,
		taskId:       atomic.Int64{},
		taskQueue:    make(chan Task, nReduce),
		taskProgress: make(map[int]chan struct{}),
		wg:           sync.WaitGroup{},
	}
	// 2. 将获取到的文件以任务队列的方式存储，为 worker 领取任务做准备
	c.wg.Add(len(files))
	go func(files []string, nReduce int) {
		// 2.1. 将所有文件分配给 map 任务
		for _, filename := range files {
			mapTask := Task{
				TaskId:   int(c.taskId.Load()),
				TaskType: MapTaskType,
				Filename: filename,
				ReduceId: nReduce,
			}
			c.taskId.Add(1)
			c.taskQueue <- mapTask
		}
		// 2.2. 等待 map 任务执行完毕
		c.wg.Wait()
		// 2.3. map 任务均执行完毕，则开启 reduce 任务
		c.phase = ReadyForReducing
		c.wg.Add(nReduce)
		// 2.4. 分配 reduce 任务并确认 reduce worker 的运行 ID
		for i := 0; i < nReduce; i++ {
			reduceTask := Task{
				TaskId:   int(c.taskId.Load()),
				TaskType: ReduceTaskType,
				ReduceId: i,
			}
			c.taskId.Add(1)
			c.taskQueue <- reduceTask
		}
		// 2.5. 确保所有 reduce 任务执行完毕，更新 coordinator 状态为执行完毕
		c.wg.Wait()
		c.phase = AllTasksComplete
	}(files, nReduce)
	// 3. 开启 socket 监听
	c.server()
	return &c
}
