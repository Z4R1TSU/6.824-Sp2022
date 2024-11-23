# 6.824: Distributed Systems

## lab1: MapReduce

1. The coordinator should notice if a worker hasn't completed its task in **10 seconds**, and give the same task to a different worker. （这里暂时不考虑幂等性，即运行相同的任务多次得到的结果仍跟运行一次得到的结果相同）
2. The worker implementation should put the output of the `X`'th reduce task in the file `mr-out-X`.
3. `mr-out-X`'s format should be generated with the Go `%v %v` format, called with the key and value every line. (See detail format in `main/mrsequential.go`)
4. The worker should put intermediate Map output in files in the current directory. 即模拟 “中间结果要保存在worker的本地磁盘上，以便于在reduce阶段进行处理”。
5. `mr/coordinator.go` needs to implement `Done()` method that returns true when the MapReduce job is complete. At that point, the `mrcoordinator.go` can exit.
6. Use the return value from `call()` to exit worker processes when the job is completely finished.
7. if the worker fails to contact the coordinator (coordinator didn't response), it can assume that the coordinator has exited because the job is done, and so the worker can terminate too.
8. Naming convention for intermediate files should be `mr-X-Y`, where `X` is the **Map** task number, and `Y` is the **reduce** task number.
9. The worker's map task code will need a way to store intermediate key/value pairs in files by using Go's `encoding/json` package.
    ```go
    // write key/value pairs in JSON format to an open file
    enc := json.NewEncoder(file)
    for _, kv := ... {
        err := enc.Encode(&kv)
    }
    // read key/value pairs from an open file
    dec := json.NewDecoder(file)
    for {
        var kv KeyValue
        if err := dec.Decode(&kv); err != nil {
            break
        }
        kva = append(kva, kv)
    }
    ```
10. The map part of your worker can use the `ihash(key)` function (in `worker.go`) to pick the reduce task for a given key.
11. Don't forget to **lock shared data** of coordinator for concurrency.
12. Wait!!! Use `time.Sleep()` or `sync.Cond`.
    1.  Worker send heartbeats to coordinator periodically.
    2.  Reduces will wait until all map tasks are completed.
13. Use `mrapps/crash.go` to test crash recovery by randomly exit map or reduce tasks.
14. Use `ioutil.TempFile` to create a temporary file and `os.Rename` to atomically rename it in case of crash.
