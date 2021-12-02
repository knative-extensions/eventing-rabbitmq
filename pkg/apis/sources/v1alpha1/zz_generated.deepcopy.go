//go:build !ignore_autogenerated
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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	v1 "knative.dev/pkg/apis/duck/v1"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RabbitmqChannelConfigSpec) DeepCopyInto(out *RabbitmqChannelConfigSpec) {
	*out = *in
	if in.PrefetchCount != nil {
		in, out := &in.PrefetchCount, &out.PrefetchCount
		*out = new(int)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RabbitmqChannelConfigSpec.
func (in *RabbitmqChannelConfigSpec) DeepCopy() *RabbitmqChannelConfigSpec {
	if in == nil {
		return nil
	}
	out := new(RabbitmqChannelConfigSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RabbitmqSource) DeepCopyInto(out *RabbitmqSource) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RabbitmqSource.
func (in *RabbitmqSource) DeepCopy() *RabbitmqSource {
	if in == nil {
		return nil
	}
	out := new(RabbitmqSource)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *RabbitmqSource) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RabbitmqSourceExchangeConfigSpec) DeepCopyInto(out *RabbitmqSourceExchangeConfigSpec) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RabbitmqSourceExchangeConfigSpec.
func (in *RabbitmqSourceExchangeConfigSpec) DeepCopy() *RabbitmqSourceExchangeConfigSpec {
	if in == nil {
		return nil
	}
	out := new(RabbitmqSourceExchangeConfigSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RabbitmqSourceList) DeepCopyInto(out *RabbitmqSourceList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]RabbitmqSource, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RabbitmqSourceList.
func (in *RabbitmqSourceList) DeepCopy() *RabbitmqSourceList {
	if in == nil {
		return nil
	}
	out := new(RabbitmqSourceList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *RabbitmqSourceList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RabbitmqSourceQueueConfigSpec) DeepCopyInto(out *RabbitmqSourceQueueConfigSpec) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RabbitmqSourceQueueConfigSpec.
func (in *RabbitmqSourceQueueConfigSpec) DeepCopy() *RabbitmqSourceQueueConfigSpec {
	if in == nil {
		return nil
	}
	out := new(RabbitmqSourceQueueConfigSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RabbitmqSourceSpec) DeepCopyInto(out *RabbitmqSourceSpec) {
	*out = *in
	in.User.DeepCopyInto(&out.User)
	in.Password.DeepCopyInto(&out.Password)
	in.ChannelConfig.DeepCopyInto(&out.ChannelConfig)
	out.ExchangeConfig = in.ExchangeConfig
	out.QueueConfig = in.QueueConfig
	if in.Sink != nil {
		in, out := &in.Sink, &out.Sink
		*out = new(v1.Destination)
		(*in).DeepCopyInto(*out)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RabbitmqSourceSpec.
func (in *RabbitmqSourceSpec) DeepCopy() *RabbitmqSourceSpec {
	if in == nil {
		return nil
	}
	out := new(RabbitmqSourceSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RabbitmqSourceStatus) DeepCopyInto(out *RabbitmqSourceStatus) {
	*out = *in
	in.SourceStatus.DeepCopyInto(&out.SourceStatus)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RabbitmqSourceStatus.
func (in *RabbitmqSourceStatus) DeepCopy() *RabbitmqSourceStatus {
	if in == nil {
		return nil
	}
	out := new(RabbitmqSourceStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SecretValueFromSource) DeepCopyInto(out *SecretValueFromSource) {
	*out = *in
	if in.SecretKeyRef != nil {
		in, out := &in.SecretKeyRef, &out.SecretKeyRef
		*out = new(corev1.SecretKeySelector)
		(*in).DeepCopyInto(*out)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SecretValueFromSource.
func (in *SecretValueFromSource) DeepCopy() *SecretValueFromSource {
	if in == nil {
		return nil
	}
	out := new(SecretValueFromSource)
	in.DeepCopyInto(out)
	return out
}
