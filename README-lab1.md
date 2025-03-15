# lab1 MapReduce

## 规则

1. 如果 worker 没有在 10s 内返回，那么 master 会认为 worker 挂掉，需要重新分配任务。（至少一次 or 恰好执行一次）
2. `pg-*.txt` 相当于输入，是需要被执行分布式 word count 的文件，而 `mr-out*` 则是结果。
3. 将 `mr/coordinator.go` 中的 Done 函数中 `ret` 改为 true 可在执行 coordinator 后立即退出。
4. 在构造 coordinator 时，传入的 `nReduce` 代表了会有 `nReduce` 个做 reduce 的 worker。
5. 第 k 个 reduce worker 的输出文件需要被命名为 `mr-out-k`，其中格式是 `%v %v` 的键值对。
6. coordinator 的 Done 函数返回 true 则 coordinator 终止，而 worker 向 coordinator 发送 `call()` 若没回应也终止，最终整个 mapreduce 流程结束。
7. 当调用 workers 的 `Worker()`时， worker 会向 coordinator 发送 RPC 索要任务，收到需要处理的**文件名**后开始执行。
8. 对于 map 阶段过后的中间文件（intermediate files）需要被命名为 `mr-X-Y`，其中 X 为 map 的任务编号，Y 为 reduce 的。
9. 读写中间文件可以参考 lab 文档，具体是 `encoding/json` 包。
10. map 阶段的 worker 可以采用 `ihash(key)` 来选取处理 reduce 的 worker。
11. 使用临时文件来避免 crash 的情况。具体地，先用`ioutil.TempFile`创建临时文件，等成功再用`os.Rename`修改成所需的最终文件名。

## 运行流程

1. 为 `MakeCoordinator()` 传入要被处理的文件和预计处理的 reduce worker 的数量，并开启 socket 监听，以启动整个 MapReduce 程序。
2. workers 调用 `Worker()` **RPC** 向 coordinator 索要任务，后者分配任务对应的文件，worker 获取之并执行 map。
3. coordinator 把对应的任务丢出去并记录任务和 worker 的对应关系，同时开启一个 goroutine 计时，若超过 10s 则重新分配该任务给其他 worker。
4. worker 正式执行 map 任务，先创建临时文件，调用 mapf 将处理结果存入 intermediate 变量，写入文件后改名（用 `ihash(key) 指定要处理的 reduce worker），最后需要给 coordinator 发送一个 **RPC** 表示执行完毕。
5. coordinator 接收到任务被完成的信号，将 map 阶段被完成的任务，用 **RPC** 按中间文件名丢给对应的 reduce worker 执行。
6. worker 正式执行 reduce 任务，先创建临时文件，调用 reducef 将结果存入结果集后写入文件并改名，返回一个 **RPC** 表示执行完毕。
7. coordinator 等待所有任务都处于 reduce 执行完毕的状态后执行 `Done()` 来结束自己。
8. worker 定时向 coordinator 发送 `call()`，若得到回复（true）则继续，否则终止自身。

## 想法

1. 出于效率考虑，使用条件变量 `sync.Cond` 来代替 Sleep。 
2. RPC 的 IDL 的规范：入参和返回结构体定义在 `rpc.go`，而具体的 Service 定义在 `coordinator.go` 中。
3. 为什么不传入执行 map 的 worker 数量？因为当我们开启 coordinator 进程后，并不依赖它来启动并组织 worker 去执行 map，而是我们需要自行启动 worker 集群，而让 coordinator 监听并接受请求，被动布置任务。
4. **重要**：确认哪些地方需要用到 RPC（有 RPC 就需要显式定义出来）？worker 和 worker 间，worker 和 coordinator 间。
5. map 阶段应该选择哪个 worker 来执行 map 任务？同 3，不是 coordinator 选，而是 worker 去领取。
6. 总结需要哪些 RPC？
   1. worker 向 coordinator 索要任务：输入无，返回任务
   2. worker 告知任务执行完毕：输入任务 ID，返回无
7. 如何构造 coordinator？
   1. 初始要被执行 word count 的文件名
   2. nReduce
   3. 当前执行的阶段
   4. 任务分配的并发控制
8. 如何构造 worker？打工人不语，只是埋头苦干
9. 如何构造 task？
   1. 任务 ID
   2. 任务类型：map 或 reduce
   3. 要处理的文件名：map 任务专属
   4. 被分配到的 reduce ID：reduce 任务专属