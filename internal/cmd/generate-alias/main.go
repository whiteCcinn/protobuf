// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:generate go run . -execute

package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/golang/protobuf/proto"
	gengo "github.com/whiteCcinn/protobuf-go/cmd/protoc-gen-go/internal_gengo"
	"github.com/whiteCcinn/protobuf-go/compiler/protogen"
	"github.com/whiteCcinn/protobuf-go/reflect/protodesc"
	"github.com/whiteCcinn/protobuf-go/reflect/protoreflect"

	"github.com/whiteCcinn/protobuf-go/types/descriptorpb"
	"github.com/whiteCcinn/protobuf-go/types/known/anypb"
	"github.com/whiteCcinn/protobuf-go/types/known/durationpb"
	"github.com/whiteCcinn/protobuf-go/types/known/emptypb"
	"github.com/whiteCcinn/protobuf-go/types/known/structpb"
	"github.com/whiteCcinn/protobuf-go/types/known/timestamppb"
	"github.com/whiteCcinn/protobuf-go/types/known/wrapperspb"
	"github.com/whiteCcinn/protobuf-go/types/pluginpb"
)

func main() {
	run := flag.Bool("execute", false, "Write generated files to destination.")
	flag.Parse()

	// Set of generated proto packages to forward to v2.
	files := []struct {
		oldGoPkg string
		newGoPkg string
		pbDesc   protoreflect.FileDescriptor
	}{{
		oldGoPkg: "github.com/golang/protobuf/protoc-gen-go/descriptor;descriptor",
		newGoPkg: "github.com/whiteCcinn/protobuf-go/types/descriptorpb",
		pbDesc:   descriptorpb.File_google_protobuf_descriptor_proto,
	}, {
		oldGoPkg: "github.com/golang/protobuf/protoc-gen-go/plugin;plugin_go",
		newGoPkg: "github.com/whiteCcinn/protobuf-go/types/pluginpb",
		pbDesc:   pluginpb.File_google_protobuf_compiler_plugin_proto,
	}, {
		oldGoPkg: "github.com/golang/protobuf/ptypes/any;any",
		newGoPkg: "github.com/whiteCcinn/protobuf-go/types/known/anypb",
		pbDesc:   anypb.File_google_protobuf_any_proto,
	}, {
		oldGoPkg: "github.com/golang/protobuf/ptypes/duration;duration",
		newGoPkg: "github.com/whiteCcinn/protobuf-go/types/known/durationpb",
		pbDesc:   durationpb.File_google_protobuf_duration_proto,
	}, {
		oldGoPkg: "github.com/golang/protobuf/ptypes/timestamp;timestamp",
		newGoPkg: "github.com/whiteCcinn/protobuf-go/types/known/timestamppb",
		pbDesc:   timestamppb.File_google_protobuf_timestamp_proto,
	}, {
		oldGoPkg: "github.com/golang/protobuf/ptypes/wrappers;wrappers",
		newGoPkg: "github.com/whiteCcinn/protobuf-go/types/known/wrapperspb",
		pbDesc:   wrapperspb.File_google_protobuf_wrappers_proto,
	}, {
		oldGoPkg: "github.com/golang/protobuf/ptypes/struct;structpb",
		newGoPkg: "github.com/whiteCcinn/protobuf-go/types/known/structpb",
		pbDesc:   structpb.File_google_protobuf_struct_proto,
	}, {
		oldGoPkg: "github.com/golang/protobuf/ptypes/empty;empty",
		newGoPkg: "github.com/whiteCcinn/protobuf-go/types/known/emptypb",
		pbDesc:   emptypb.File_google_protobuf_empty_proto,
	}}

	// For each package, construct a proto file that public imports the package.
	var req pluginpb.CodeGeneratorRequest
	var flags []string
	for _, file := range files {
		pkgPath := file.oldGoPkg[:strings.IndexByte(file.oldGoPkg, ';')]
		fd := &descriptorpb.FileDescriptorProto{
			Name:             proto.String(pkgPath + "/" + path.Base(pkgPath) + ".proto"),
			Syntax:           proto.String(file.pbDesc.Syntax().String()),
			Dependency:       []string{file.pbDesc.Path()},
			PublicDependency: []int32{0},
			Options:          &descriptorpb.FileOptions{GoPackage: proto.String(file.oldGoPkg)},
		}
		req.ProtoFile = append(req.ProtoFile, protodesc.ToFileDescriptorProto(file.pbDesc), fd)
		req.FileToGenerate = append(req.FileToGenerate, fd.GetName())
		flags = append(flags, "M"+file.pbDesc.Path()+"="+file.newGoPkg)
	}
	req.Parameter = proto.String(strings.Join(flags, ","))

	// Use the internal logic of protoc-gen-go to generate the files.
	gen, err := protogen.Options{}.New(&req)
	check(err)
	for _, file := range gen.Files {
		if file.Generate {
			gengo.GenerateVersionMarkers = false
			gengo.GenerateFile(gen, file)
		}
	}

	// Write the generated files.
	resp := gen.Response()
	if resp.Error != nil {
		panic("gengo error: " + resp.GetError())
	}
	for _, file := range resp.File {
		relPath, err := filepath.Rel(filepath.FromSlash("github.com/golang/protobuf"), file.GetName())
		check(err)

		check(ioutil.WriteFile(relPath+".bak", []byte(file.GetContent()), 0664))
		if *run {
			fmt.Println("#", relPath)
			check(os.Rename(relPath+".bak", relPath))
		} else {
			cmd := exec.Command("diff", relPath, relPath+".bak", "-N", "-u")
			cmd.Stdout = os.Stdout
			cmd.Run()
			os.Remove(relPath + ".bak") // best-effort delete
		}
	}
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}
