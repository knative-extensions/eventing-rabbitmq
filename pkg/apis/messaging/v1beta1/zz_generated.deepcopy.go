// +build !ignore_autogenerated

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
// Code generated by deepcopy-gen. DO NOT EDIT.

package v1beta1

import (
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RabbitmqChannel) DeepCopyInto(out *RabbitmqChannel) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RabbitmqChannel.
func (in *RabbitmqChannel) DeepCopy() *RabbitmqChannel {
	if in == nil {
		return nil
	}
	out := new(RabbitmqChannel)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *RabbitmqChannel) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RabbitmqChannelList) DeepCopyInto(out *RabbitmqChannelList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]RabbitmqChannel, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RabbitmqChannelList.
func (in *RabbitmqChannelList) DeepCopy() *RabbitmqChannelList {
	if in == nil {
		return nil
	}
	out := new(RabbitmqChannelList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *RabbitmqChannelList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RabbitmqChannelSpec) DeepCopyInto(out *RabbitmqChannelSpec) {
	*out = *in
	out.Config = in.Config
	in.ChannelableSpec.DeepCopyInto(&out.ChannelableSpec)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RabbitmqChannelSpec.
func (in *RabbitmqChannelSpec) DeepCopy() *RabbitmqChannelSpec {
	if in == nil {
		return nil
	}
	out := new(RabbitmqChannelSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RabbitmqChannelStatus) DeepCopyInto(out *RabbitmqChannelStatus) {
	*out = *in
	in.ChannelableStatus.DeepCopyInto(&out.ChannelableStatus)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RabbitmqChannelStatus.
func (in *RabbitmqChannelStatus) DeepCopy() *RabbitmqChannelStatus {
	if in == nil {
		return nil
	}
	out := new(RabbitmqChannelStatus)
	in.DeepCopyInto(out)
	return out
}
