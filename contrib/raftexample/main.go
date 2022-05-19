// Copyright 2015 The etcd Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"flag"
	"strings"

	"go.etcd.io/etcd/raft/v3/raftpb"
)

func main() {
	cluster := flag.String("cluster", "http://127.0.0.1:9021", "comma separated cluster peers")
	id := flag.Int("id", 1, "node ID")
	kvport := flag.Int("port", 9121, "key-value server port")
	join := flag.Bool("join", false, "join an existing cluster")
	flag.Parse()

	// 这个chan是传递写日志信息用的
	proposeC := make(chan string)
	defer close(proposeC)
	// 用于raft这是信息修改
	confChangeC := make(chan raftpb.ConfChange)
	defer close(confChangeC)

	// raft provides a commit stream for the proposals from the http api
	var kvs *kvstore
	getSnapshot := func() ([]byte, error) { return kvs.getSnapshot() }
	// newRaftNode 从名字来看，是创建了一个新的Raft node，自然要知道和这个node通信的chan，所以返回前两者
	// snapshotterReady
	// 传入参数 getSnapshot 是让node从http api中拿kv数据
	commitC, errorC, snapshotterReady := newRaftNode(*id, strings.Split(*cluster, ","), *join, getSnapshot, proposeC, confChangeC)

	// 拿到raft返回信息的commitC和errorC chan后，监听这两个chan
	// TODO <-snapshotterReady,是指这个参数从这个chan拿，拿不到会阻塞
	// 这样就实现了让raft加载一半以后，开始创建kv----太牛了吧学到了
	// 但这是我猜的，但应该就是无缓冲区的chan的原理实现
	kvs = newKVStore(<-snapshotterReady, proposeC, commitC, errorC)

	// the key-value http handler will propose updates to raft
	serveHttpKVAPI(kvs, *kvport, confChangeC, errorC)
}
