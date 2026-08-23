# 实验方案

## 1. 低延迟实验

保持相同机器和相同Docker资源配置，分别运行：

```bash
go run ./ue -action benchmark -count 100 -concurrency 1 -scenario stable -output results/c1.csv
go run ./ue -action benchmark -count 500 -concurrency 10 -scenario stable -output results/c10.csv
go run ./ue -action benchmark -count 1000 -concurrency 50 -scenario stable -output results/c50.csv
go run ./ue -action benchmark -count 2000 -concurrency 100 -scenario stable -output results/c100.csv
```

记录平均延迟、P95、P99、吞吐量和成功率。每组至少重复三次，报告中使用均值，并说明机器型号、CPU、内存、Go版本和运行方式。

不要把结果写成“达到5G/6G空口低延迟指标”，应表述为“LiteCore软件控制面端到端注册延迟”。

## 2. 信道感知实验

对stable、edge、degrading和mixed场景各运行1000个UE，固定`-seed 42`。比较接入成功率，解释信号功率/SINR阈值如何影响结果。

## 3. 可靠性实验

### 下游不可用

停止SMF后发起注册，预期AMF返回Unavailable而非崩溃；恢复SMF后再次注册应成功。

### 幂等性

同一个UE连续注册两次，预期session_id和ue_ip一致；连续注销两次均成功。

### IP回收

将`IP_POOL_SIZE`设为1：UE-A注册后UE-B失败；注销UE-A后UE-B成功并获得回收的IP。

### Kubernetes恢复

持续发送请求时删除一个Pod，记录Kubernetes重建Pod所需时间。由于当前是内存状态，重建后原状态丢失必须如实记录；该实验验证的是进程恢复，不是状态无损高可用。

## 4. 建议图表

- 并发数—平均/P95/P99延迟折线图
- 并发数—吞吐量折线图
- 四种信道场景—接入成功率柱状图
- 故障发生前后—成功率时间序列图
