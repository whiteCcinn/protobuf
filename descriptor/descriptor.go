// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package descriptor provides functions for obtaining the protocol buffer
// descriptors of generated Go types.
//
// Deprecated: See the "github.com/whiteCcinn/protobuf-go/reflect/protoreflect" package
// for how to obtain an EnumDescriptor or MessageDescriptor in order to
// programatically interact with the protobuf type system.
package descriptor

import (
	"bytes"
	"compress/gzip"
	"io/ioutil"
	"sync"

	"github.com/golang/protobuf/proto"
	"github.com/whiteCcinn/protobuf-go/reflect/protodesc"
	"github.com/whiteCcinn/protobuf-go/reflect/protoreflect"
	"github.com/whiteCcinn/protobuf-go/runtime/protoimpl"

	descriptorpb "github.com/golang/protobuf/protoc-gen-go/descriptor"
)

// Message is proto.Message with a method to return its descriptor.
//
// Deprecated: The Descriptor method may not be generated by future
// versions of protoc-gen-go, meaning that this interface may not
// be implemented by many concrete message types.
type Message interface {
	proto.Message
	Descriptor() ([]byte, []int)
}

// ForMessage returns the file descriptor proto containing
// the message and the message descriptor proto for the message itself.
// The returned proto messages must not be mutated.
//
// Deprecated: Not all concrete message types satisfy the Message interface.
// Use MessageDescriptorProto instead. If possible, the calling code should
// be rewritten to use protobuf reflection instead.
// See package "github.com/whiteCcinn/protobuf-go/reflect/protoreflect" for details.
func ForMessage(m Message) (*descriptorpb.FileDescriptorProto, *descriptorpb.DescriptorProto) {
	return MessageDescriptorProto(m)
}

type rawDesc struct {
	fileDesc []byte
	indexes  []int
}

var rawDescCache sync.Map // map[protoreflect.Descriptor]*rawDesc

func deriveRawDescriptor(d protoreflect.Descriptor) ([]byte, []int) {
	// Fast-path: check whether raw descriptors are already cached.
	origDesc := d
	if v, ok := rawDescCache.Load(origDesc); ok {
		return v.(*rawDesc).fileDesc, v.(*rawDesc).indexes
	}

	// Slow-path: derive the raw descriptor from the v2 descriptor.

	// Start with the leaf (a given enum or message declaration) and
	// ascend upwards until we hit the parent file descriptor.
	var idxs []int
	for {
		idxs = append(idxs, d.Index())
		d = d.Parent()
		if d == nil {
			// TODO: We could construct a FileDescriptor stub for standalone
			// descriptors to satisfy the API.
			return nil, nil
		}
		if _, ok := d.(protoreflect.FileDescriptor); ok {
			break
		}
	}

	// Obtain the raw file descriptor.
	fd := d.(protoreflect.FileDescriptor)
	b, _ := proto.Marshal(protodesc.ToFileDescriptorProto(fd))
	file := protoimpl.X.CompressGZIP(b)

	// Reverse the indexes, since we populated it in reverse.
	for i, j := 0, len(idxs)-1; i < j; i, j = i+1, j-1 {
		idxs[i], idxs[j] = idxs[j], idxs[i]
	}

	if v, ok := rawDescCache.LoadOrStore(origDesc, &rawDesc{file, idxs}); ok {
		return v.(*rawDesc).fileDesc, v.(*rawDesc).indexes
	}
	return file, idxs
}

// EnumRawDescriptor returns the GZIP'd raw file descriptor representing
// the enum and the index path to reach the enum declaration.
// The returned slices must not be mutated.
func EnumRawDescriptor(e proto.GeneratedEnum) ([]byte, []int) {
	if ev, ok := e.(interface{ EnumDescriptor() ([]byte, []int) }); ok {
		return ev.EnumDescriptor()
	}
	ed := protoimpl.X.EnumTypeOf(e)
	return deriveRawDescriptor(ed.Descriptor())
}

// MessageRawDescriptor returns the GZIP'd raw file descriptor representing
// the message and the index path to reach the message declaration.
// The returned slices must not be mutated.
func MessageRawDescriptor(m proto.GeneratedMessage) ([]byte, []int) {
	if mv, ok := m.(interface{ Descriptor() ([]byte, []int) }); ok {
		return mv.Descriptor()
	}
	md := protoimpl.X.MessageTypeOf(m)
	return deriveRawDescriptor(md.Descriptor())
}

var fileDescCache sync.Map // map[*byte]*descriptorpb.FileDescriptorProto

func deriveFileDescriptor(rawDesc []byte) *descriptorpb.FileDescriptorProto {
	// Fast-path: check whether descriptor protos are already cached.
	if v, ok := fileDescCache.Load(&rawDesc[0]); ok {
		return v.(*descriptorpb.FileDescriptorProto)
	}

	// Slow-path: derive the descriptor proto from the GZIP'd message.
	zr, err := gzip.NewReader(bytes.NewReader(rawDesc))
	if err != nil {
		panic(err)
	}
	b, err := ioutil.ReadAll(zr)
	if err != nil {
		panic(err)
	}
	fd := new(descriptorpb.FileDescriptorProto)
	if err := proto.Unmarshal(b, fd); err != nil {
		panic(err)
	}
	if v, ok := fileDescCache.LoadOrStore(&rawDesc[0], fd); ok {
		return v.(*descriptorpb.FileDescriptorProto)
	}
	return fd
}

// EnumDescriptorProto returns the file descriptor proto representing
// the enum and the enum descriptor proto for the enum itself.
// The returned proto messages must not be mutated.
func EnumDescriptorProto(e proto.GeneratedEnum) (*descriptorpb.FileDescriptorProto, *descriptorpb.EnumDescriptorProto) {
	rawDesc, idxs := EnumRawDescriptor(e)
	if rawDesc == nil || idxs == nil {
		return nil, nil
	}
	fd := deriveFileDescriptor(rawDesc)
	if len(idxs) == 1 {
		return fd, fd.EnumType[idxs[0]]
	}
	md := fd.MessageType[idxs[0]]
	for _, i := range idxs[1 : len(idxs)-1] {
		md = md.NestedType[i]
	}
	ed := md.EnumType[idxs[len(idxs)-1]]
	return fd, ed
}

// MessageDescriptorProto returns the file descriptor proto representing
// the message and the message descriptor proto for the message itself.
// The returned proto messages must not be mutated.
func MessageDescriptorProto(m proto.GeneratedMessage) (*descriptorpb.FileDescriptorProto, *descriptorpb.DescriptorProto) {
	rawDesc, idxs := MessageRawDescriptor(m)
	if rawDesc == nil || idxs == nil {
		return nil, nil
	}
	fd := deriveFileDescriptor(rawDesc)
	md := fd.MessageType[idxs[0]]
	for _, i := range idxs[1:] {
		md = md.NestedType[i]
	}
	return fd, md
}
