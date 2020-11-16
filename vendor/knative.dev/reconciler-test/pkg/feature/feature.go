/*
Copyright 2020 The Knative Authors

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

package feature

import (
	"context"
	"fmt"
	"testing"
)

type Feature struct {
	Name  string
	Steps []Step
}

type StepFn func(ctx context.Context, t *testing.T)

type Step struct {
	Name string
	S    States
	L    Levels
	T    Timing
	Fn   StepFn
}

func (s *Step) TestName() string {
	switch s.T {
	case Assert:
		return fmt.Sprintf("[%s/%s]%s", s.S, s.L, s.Name)
	default:
		return s.Name
	}
}

func (f *Feature) Setup(name string, fn StepFn) {
	f.AddStep(Step{
		Name: name,
		S:    Any,
		L:    All,
		T:    Setup,
		Fn:   fn,
	})
}

func (f *Feature) Requirement(name string, fn StepFn) {
	f.AddStep(Step{
		Name: name,
		S:    Any,
		L:    All,
		T:    Requirement,
		Fn:   fn,
	})
}

func (f *Feature) Teardown(name string, fn StepFn) {
	f.AddStep(Step{
		Name: name,
		S:    Any,
		L:    All,
		T:    Teardown,
		Fn:   fn,
	})
}

func (f *Feature) AddStep(step ...Step) {
	f.Steps = append(f.Steps, step...)
}

func (a *Asserter) Assert(l Levels, name string, fn StepFn) {
	a.f.AddStep(Step{
		Name: fmt.Sprintf("%s %s", a.name, name),
		S:    a.s,
		L:    l,
		T:    Assert,
		Fn:   fn,
	})
}

type Assertable interface {
	Must(name string, fn StepFn) Assertable
	Should(name string, fn StepFn) Assertable
	May(name string, fn StepFn) Assertable
	MustNot(name string, fn StepFn) Assertable
	ShouldNot(name string, fn StepFn) Assertable
}

func (f *Feature) Alpha(name string) Assertable {
	return f.asserter(Alpha, name)
}

func (f *Feature) Beta(name string) Assertable {
	return f.asserter(Beta, name)
}

func (f *Feature) Stable(name string) Assertable {
	return f.asserter(Stable, name)
}

func (f *Feature) asserter(s States, name string) Assertable {
	return &Asserter{
		f:    f,
		name: name,
		s:    s,
	}
}

type Asserter struct {
	f    *Feature
	name string
	s    States
}

func (a *Asserter) Must(name string, fn StepFn) Assertable {
	a.Assert(Must, name, fn)
	return a
}

func (a *Asserter) Should(name string, fn StepFn) Assertable {
	a.Assert(Should, name, fn)
	return a
}

func (a *Asserter) May(name string, fn StepFn) Assertable {
	a.Assert(May, name, fn)
	return a
}

func (a *Asserter) MustNot(name string, fn StepFn) Assertable {
	a.Assert(MustNot, name, fn)
	return a
}

func (a *Asserter) ShouldNot(name string, fn StepFn) Assertable {
	a.Assert(ShouldNot, name, fn)
	return a
}
