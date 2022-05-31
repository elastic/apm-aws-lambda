// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package model

// Service represents the service handling transactions being traced.
type Service struct {
	// Name is the immutable name of the service.
	Name string `json:"name,omitempty"`

	// Version is the version of the service, if it has one.
	Version string `json:"version,omitempty"`

	// Environment is the name of the service's environment, if it has
	// one, e.g. "production" or "staging".
	Environment string `json:"environment,omitempty"`

	// Agent holds information about the Elastic APM agent tracing this
	// service's transactions.
	Agent Agent `json:"agent,omitempty"`

	// Framework holds information about the service's framework, if any.
	Framework Framework `json:"framework,omitempty"`

	// Language holds information about the programming language in which
	// the service is written.
	Language Language `json:"language,omitempty"`

	// Runtime holds information about the programming language runtime
	// running this service.
	Runtime Runtime `json:"runtime,omitempty"`

	// Node holds unique information about each service node
	Node ServiceNode `json:"node,omitempty"`
}

// Agent holds information about the Elastic APM agent.
type Agent struct {
	// Name is the name of the Elastic APM agent, e.g. "Go".
	Name string `json:"name"`

	// Version is the version of the Elastic APM agent, e.g. "1.0.0".
	Version string `json:"version"`
}

// Framework holds information about the framework (typically web)
// used by the service.
type Framework struct {
	// Name is the name of the framework.
	Name string `json:"name"`

	// Version is the version of the framework.
	Version string `json:"version"`
}

// Language holds information about the programming language used.
type Language struct {
	// Name is the name of the programming language.
	Name string `json:"name"`

	// Version is the version of the programming language.
	Version string `json:"version,omitempty"`
}

// Runtime holds information about the programming language runtime.
type Runtime struct {
	// Name is the name of the programming language runtime.
	Name string `json:"name"`

	// Version is the version of the programming language runtime.
	Version string `json:"version"`
}

// ServiceNode holds unique information about each service node
type ServiceNode struct {
	// ConfiguredName holds the name of the service node
	ConfiguredName string `json:"configured_name,omitempty"`
}

// System represents the system (operating system and machine) running the
// service.
type System struct {
	// Architecture is the system's hardware architecture.
	Architecture string `json:"architecture,omitempty"`

	// Hostname is the system's hostname.
	Hostname string `json:"hostname,omitempty"`

	// Platform is the system's platform, or operating system name.
	Platform string `json:"platform,omitempty"`

	// Container describes the container running the service.
	Container Container `json:"container,omitempty"`

	// Kubernetes describes the kubernetes node and pod running the service.
	Kubernetes Kubernetes `json:"kubernetes,omitempty"`
}

// Process represents an operating system process.
type Process struct {
	// Pid is the process ID.
	Pid int `json:"pid"`

	// Ppid is the parent process ID, if known.
	Ppid *int `json:"ppid,omitempty"`

	// Title is the title of the process.
	Title string `json:"title,omitempty"`

	// Argv holds the command line arguments used to start the process.
	Argv []string `json:"argv,omitempty"`
}

// Container represents the container (e.g. Docker) running the service.
type Container struct {
	// ID is the unique container ID.
	ID string `json:"id"`
}

// Kubernetes describes properties of the Kubernetes node and pod in which
// the service is running.
type Kubernetes struct {
	// Namespace names the Kubernetes namespace in which the pod exists.
	Namespace string `json:"namespace,omitempty"`

	// Node describes the Kubernetes node running the service's pod.
	Node KubernetesNode `json:"node,omitempty"`

	// Pod describes the Kubernetes pod running the service.
	Pod KubernetesPod `json:"pod,omitempty"`
}

// KubernetesNode describes a Kubernetes node.
type KubernetesNode struct {
	// Name holds the node name.
	Name string `json:"name,omitempty"`
}

// KubernetesPod describes a Kubernetes pod.
type KubernetesPod struct {
	// Name holds the pod name.
	Name string `json:"name,omitempty"`

	// UID holds the pod UID.
	UID string `json:"uid,omitempty"`
}

// Cloud represents the cloud in which the service is running.
type Cloud struct {
	// Provider is the cloud provider name, e.g. aws, azure, gcp.
	Provider string `json:"provider"`

	// Region is the cloud region name, e.g. us-east-1.
	Region string `json:"region,omitempty"`

	// AvailabilityZone is the cloud availability zone name, e.g. us-east-1a.
	AvailabilityZone string `json:"availability_zone,omitempty"`

	// Instance holds information about the cloud instance (virtual machine).
	Instance CloudInstance `json:"instance,omitempty"`

	// Machine also holds information about the cloud instance (virtual machine).
	Machine CloudMachine `json:"machine,omitempty"`

	// Account holds information about the cloud account.
	Account CloudAccount `json:"account,omitempty"`

	// Project holds information about the cloud project.
	Project CloudProject `json:"project,omitempty"`

	Service CloudService `json:"service,omitempty"`
}

// CloudInstance holds information about a cloud instance (virtual machine).
type CloudInstance struct {
	// ID holds the cloud instance identifier.
	ID string `json:"id,omitempty"`

	// ID holds the cloud instance name.
	Name string `json:"name,omitempty"`
}

// CloudMachine holds information about a cloud instance (virtual machine).
type CloudMachine struct {
	// Type holds the cloud instance type, e.g. t2.medium.
	Type string `json:"type,omitempty"`
}

// CloudAccount holds information about a cloud account.
type CloudAccount struct {
	// ID holds the cloud account identifier.
	ID string `json:"id,omitempty"`

	// ID holds the cloud account name.
	Name string `json:"name,omitempty"`
}

// CloudProject holds information about a cloud project.
type CloudProject struct {
	// ID holds the cloud project identifier.
	ID string `json:"id,omitempty"`

	// Name holds the cloud project name.
	Name string `json:"name,omitempty"`
}

type CloudService struct {
	Name string `json:"name,omitempty"`
}

// User holds information about an authenticated user.
type User struct {
	// Username holds the username of the user.
	Username string `json:"username,omitempty"`

	// ID identifies the user, e.g. a primary key. This may be
	// a string or number.
	ID string `json:"id,omitempty"`

	// Email holds the email address of the user.
	Email string `json:"email,omitempty"`
}

type Metadata struct {
	Service Service                `json:"service,omitempty"`
	User    User                   `json:"user,omitempty"`
	Labels  map[string]interface{} `json:"labels,omitempty"`
	Process Process                `json:"process,omitempty"`
	System  System                 `json:"system,omitempty"`
	Cloud   Cloud                  `json:"cloud,omitempty"`
}
