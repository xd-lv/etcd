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

package raft

import (
	"errors"

	pb "go.etcd.io/etcd/raft/v3/raftpb"
)

// Bootstrap initializes the RawNode for first use by appending configuration
// changes for the supplied peers. This method returns an error if the Storage
// is nonempty.
// TODO 为啥 storage必须为空，因为是初始化？
//
// It is recommended that instead of calling this method, applications bootstrap
// their state manually by setting up a Storage that has a first index > 1 and
// which stores the desired ConfState as its InitialState.
func (rn *RawNode) Bootstrap(peers []Peer) error {
	if len(peers) == 0 {
		return errors.New("must provide at least one peer to Bootstrap")
	}
	// lastIndex 从那个拼接图里的示意图
	lastIndex, err := rn.raft.raftLog.storage.LastIndex()
	if err != nil {
		return err
	}

	if lastIndex != 0 {
		return errors.New("can't bootstrap a nonempty Storage")
	}

	// We've faked out initial entries above, but nothing has been
	// persisted. Start with an empty HardState (thus the first Ready will
	// emit a HardState update for the app to persist).
	// 因为是空的，所以之前已经持久化的数据本身就是不存在的
	rn.prevHardSt = emptyState

	// TODO(tbg): remove StartNode and give the application the right tools to
	// bootstrap the initial membership in a cleaner way.
	rn.raft.becomeFollower(1, None)
	// 添加peer，本事就是一种entries，所以要保存记录
	ents := make([]pb.Entry, len(peers))
	for i, peer := range peers {
		cc := pb.ConfChange{Type: pb.ConfChangeAddNode, NodeID: peer.ID, Context: peer.Context}
		data, err := cc.Marshal()
		if err != nil {
			return err
		}

		ents[i] = pb.Entry{Type: pb.EntryConfChange, Term: 1, Index: uint64(i + 1), Data: data}
	}
	// entries添加到ents
	rn.raft.raftLog.append(ents...)

	// Now apply them, mainly so that the application can call Campaign
	// immediately after StartNode in tests. Note that these nodes will
	// be added to raft twice: here and when the application's Ready
	// loop calls ApplyConfChange. The calls to addNode must come after
	// all calls to raftLog.append so progress.next is set after these
	// bootstrapping entries (it is an error if we try to append these
	// entries since they have already been committed).
	// We do not set raftLog.applied so the application will be able
	// to observe all conf changes via Ready.CommittedEntries.
	//
	// TODO(bdarnell): These entries are still unstable; do we need to preserve
	// the invariant that committed < unstable?
	// commited：提交到local的就是commited，持久化的就是applied
	// 所以这里是合理的
	rn.raft.raftLog.committed = uint64(len(ents))
	for _, peer := range peers {
		// 更新这些设置的变化
		// 每添加一个peer，肯定都是一种变化
		rn.raft.applyConfChange(pb.ConfChange{NodeID: peer.ID, Type: pb.ConfChangeAddNode}.AsV2())
	}
	return nil
}
