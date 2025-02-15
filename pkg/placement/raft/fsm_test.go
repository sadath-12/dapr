/*
Copyright 2021 The Dapr Authors
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package raft

import (
	"bytes"
	"io"
	"testing"

	"github.com/hashicorp/raft"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFSMApply(t *testing.T) {
	fsm := newFSM()

	t.Run("upsertMember", func(t *testing.T) {
		cmdLog, err := makeRaftLogCommand(MemberUpsert, DaprHostMember{
			Name:     "127.0.0.1:3030",
			AppID:    "fakeAppID",
			Entities: []string{"actorTypeOne", "actorTypeTwo"},
		})

		require.NoError(t, err)

		raftLog := &raft.Log{
			Index: 1,
			Term:  1,
			Type:  raft.LogCommand,
			Data:  cmdLog,
		}

		resp := fsm.Apply(raftLog)
		updated, ok := resp.(bool)

		assert.True(t, ok)
		assert.True(t, updated)
		assert.Equal(t, uint64(1), fsm.state.TableGeneration())
		assert.Len(t, fsm.state.Members(), 1)
	})

	t.Run("removeMember", func(t *testing.T) {
		cmdLog, err := makeRaftLogCommand(MemberRemove, DaprHostMember{
			Name: "127.0.0.1:3030",
		})

		require.NoError(t, err)

		raftLog := &raft.Log{
			Index: 2,
			Term:  1,
			Type:  raft.LogCommand,
			Data:  cmdLog,
		}

		resp := fsm.Apply(raftLog)
		updated, ok := resp.(bool)

		assert.True(t, ok)
		assert.True(t, updated)
		assert.Equal(t, uint64(2), fsm.state.TableGeneration())
		assert.Empty(t, fsm.state.Members())
	})
}

func TestRestore(t *testing.T) {
	// arrange
	fsm := newFSM()

	s := newDaprHostMemberState()
	s.upsertMember(&DaprHostMember{
		Name:     "127.0.0.1:8080",
		AppID:    "FakeID",
		Entities: []string{"actorTypeOne", "actorTypeTwo"},
	})
	buf := bytes.NewBuffer(make([]byte, 0, 256))
	err := s.persist(buf)
	require.NoError(t, err)

	// act
	err = fsm.Restore(io.NopCloser(buf))

	// assert
	require.NoError(t, err)
	assert.Len(t, fsm.State().Members(), 1)
	assert.Len(t, fsm.State().hashingTableMap(), 2)
}

func TestPlacementState(t *testing.T) {
	fsm := newFSM()
	m := DaprHostMember{
		Name:     "127.0.0.1:3030",
		AppID:    "fakeAppID",
		Entities: []string{"actorTypeOne", "actorTypeTwo"},
	}
	cmdLog, err := makeRaftLogCommand(MemberUpsert, m)
	require.NoError(t, err)

	fsm.Apply(&raft.Log{
		Index: 1,
		Term:  1,
		Type:  raft.LogCommand,
		Data:  cmdLog,
	})

	newTable := fsm.PlacementState()
	assert.Equal(t, "1", newTable.GetVersion())
	assert.Len(t, newTable.GetEntries(), 2)
}
