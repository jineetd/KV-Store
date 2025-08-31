# Workshop 6/Systems Workshop 1: Master Node in Map Reduce 

In this module of the class, you are going to implement the base code for a fault-tolerant Master in the MapReduce framework. Additionally, you are going to create the handlers, interfaces, and scoreboard required for the Master. You will be using Docker containers as nodes, with Go or C++ as your implementation language, and Kubernetes to orchestrate it all.

The repository specific setup instructions are located in [Appendix A](#appendix-a--repo-specific-instructions).

<details>
<summary>Table of Contents</summary>

1. [Expected outcome](#1-expected-outcome)
2. [Assumptions](#2-assumptions)
3. [Background Information](#3-background-information)
    1. [Kubernetes](#31-kubernetes)
        1. [Helm and KIND](#311-helm-and-kind)
    2. [Golang](#32-golang)
        1. [Go Installation](#321-go-installation)
        2. [Go Modules and Dependency Management System](#322-go-modules-and-dependency-management-system)
    3. [Useful References](#33-useful-references)
4. [Specification](#4-specification)
5. [Download Repo and Installation](#5-download-repo-and-installation)
    1. [C++ Users](#501-c-users)
    2. [Go Users](#502-go-users)
6. [Implementation](#6-implementation)
    1. [Setting up a gRPC Application](#61-setting-up-a-grpc-application)
    2. [Implementing Leader Election](#62-implementing-leader-election)
        1. [Using etcd — C++ and Go](#621-using-etcd--c-and-go)
        2. [Using ZooKeeper — C++ only](#622-using-zookeeper--c-only)
    3. [Building Containers](#63-building-containers)
        1. [Building Containers with C++](#631-building-containers-with-c)
    4. [Deploying Containers with Kubernetes](#64-deploying-containers-with-kubernetes)
7. [Useful References](#7-useful-references)
8. [Deliverables](#8-deliverables)
    1. [Demo](#81-demo)
- [Appendix A — Repo Specific Instructions](#appendix-a--repo-specific-instructions)
    - [Installation for C++](#installation-for-c)
    - [Directory Structure for C++ Repo](#directory-structure-for-c-repo)
    - [Installation for Go](#installation-for-go)
    - [Directory Structure for Go Repo](#directory-structure-for-go-repo)

</details>


## 1. Expected outcome
The student will learn about:
- The data structures associated with the Master of the MapReduce framework.
- Implementing remote procedure calls (RPC) to execute code on remote computers (virtual machines), using the gRPC library.
- Leader election using etcd/ZooKeeper.
- Run framework on containers built with Docker and deploy the containers using Kubernetes.
Specifically, you will:
    1. Develop gRPC client and server applications in the Go or C++ programming languages.
    2. Implement leader election in the applications using the distributed data store etcd or ZooKeeper.
    3. Develop the applications to run on containers built with Docker and deploy the containers using Kubernetes.

## 2. Assumptions
This workshop assumes that the student knows how to program in Go or C++. The workshop also assumes the student is using a computer with Ubuntu installed (or a comparable virtual machine).

## 3. Background Information
This section goes through some basic concepts in Kubernetes, Helm and KIND that may be helpful for this module. It also briefly walks through setting up a Go environment for this module (applicable to those who choose to implement in Go). If you are familiar with these technologies, feel free to skip to [section 4](#4-specification).

### 3.1 Kubernetes
In the NFV workshop, you used Docker containers as nodes in a network, utilizing it as a lightweight VM. While this is sufficient for running single containers and a non complex system, for a distributed system that needs multiple containers running at once with replication, failures, and communication between each other, we would need some system to coordinate and orchestrate that. A specific example of this would be the orchestrator you've built for the NFV project.

This is where Kubernetes comes in. Kubernetes is a service that manages automatic deployment, scaling, and management of containerized applications across a distributed system of hosts. For those who are unfamiliar with Kubernetes, it is crucial to understand how Kubernetes models an application.

<p align="center">
    <picture>
      <source media="(prefers-color-scheme: dark)" srcset="https://github.gatech.edu/cs8803-SIC/workshop6-go/assets/59780/92fe875b-319c-4183-9219-b41ee2c8480a">
      <source media="(prefers-color-scheme: light)" srcset="https://github.gatech.edu/cs8803-SIC/workshop6-go/assets/59780/1c0cd042-8244-45f7-93ef-0c99d6cf1993">
      <img  src="https://github.gatech.edu/cs8803-SIC/workshop6-go/assets/59780/1c0cd042-8244-45f7-93ef-0c99d6cf1993">
    </picture>
</p>

<p align='center'>
    <em>
	    Kubernetes Abstraction
    </em>
</p> 

The figure above shows a rough diagram of how Kubernetes functions. The lowest level of granularity in Kubernetes is a **pod**. Pods can be thought of as a single "VM" that runs one or more Docker containers. The images for containers ran in pods are pulled from either a public, private, or local **container registry**. You can think of a container registry as a repository of Docker images. Each physical node in a Kubernetes cluster can run multiple pods, which in turn, can run multiple Docker containers. For simplicity, we recommend running a single Docker container in a pod for this module. Developers can connect to a Kubernetes cluster using the kubectl command line tool. Once connected, developers can deploy their application on the cluster via the command line and a YAML configuration file.

<p align="center">
    <picture>
      <source media="(prefers-color-scheme: dark)" srcset="https://github.gatech.edu/cs8803-SIC/workshop6-go/assets/59780/0a52065a-1b3f-4053-8f8b-26f8a55ed291">
      <source media="(prefers-color-scheme: light)" srcset="https://github.gatech.edu/cs8803-SIC/workshop6-go/assets/59780/3c594084-e73b-4b8a-add9-b2f3d7269246">
      <img  src="https://github.gatech.edu/cs8803-SIC/workshop6-go/assets/59780/3c594084-e73b-4b8a-add9-b2f3d7269246">
    </picture>
</p>

<p align='center'>
    <em>
	    Kubernetes Objects
    </em>
</p> 

While the first figure explained the basic abstraction of a Kubernetes cluster, Kubernetes defines different objects that wraps around the basic concept of pods, and are used in the YAML configuration file to setup your application. The second figure (directly above) illustrates the Service, Deployment, and Replica Set objects. 

A **replica set** defines a configuration where a pod is replicated for a number of times. If a pod in a replica set dies, the Kubernetes cluster will automatically spawn a new pod. 

A **deployment** object is a more generalized object that wraps around Replica sets, and provides declarative updates to Pods along with a lot of other useful features. In general, Replica Sets are not explicitly defined in a Kubernetes configuration file, a deployment object that specifies the number of replicas for pods will automatically set up a replica set. 

Finally, a Kubernetes **service** object can connect to multiple deployments. Since pods can fail and new replica pods can be added in a deployment, it would be difficult to interact with your application with only deployments. A Kubernetes service acts as a single point of access to your application, and thus all underlying pods are clustered under a single IP address. There may be times when there is a need to access each pod underneath a service via its unique IP address. A [headless-service](https://kubernetes.io/docs/concepts/services-networking/service/#headless-services) can be used in such cases.

For this module, you will define your own Kubernetes configuration file for mapreduce. More information on how to actually write the YAML config file can be seen in this [document](https://kubernetes.io/docs/concepts/), or this [youtube video](https://www.youtube.com/watch?v=qmDzcu5uY1I&t=474s). We recommend reading through the workload and services sections of the Kubernetes document.

#### 3.1.1 Helm and KIND
Now that you have a basic understanding of Kubernetes, we'll introduce to you two different Kubernetes technologies that you will use in this module.

**Helm** is a package manager for Kubernetes. You can think of Helm as the "apt of Kubernetes". Using Helm, you can add public repositories of Kubernetes applications, which contain ready-built Kubernetes applications configs, known as "charts". You can then deploy one of these public charts directly onto your own Kubernetes cluster. We will use Helm to deploy an etcd or ZooKeeper Kubernetes service onto our cluster.

**KIND** is a local implementation of a Kubernetes cluster. Since Kubernetes is designed to run on a cluster of multiple hosts, it is somewhat difficult to work with locally since you only have one host. Some clever developers have figured out a way to simulate a distributed environment using Docker called **K**ubernetes **IN** **D**ocker (KIND). KIND will be used as your local Kubernetes cluster for you to test your code.

Helm and KIND will be installed using the provided install script.

### 3.2 Golang
#### 3.2.1 Go Installation
For this module, we will be using go version 1.17. You will have to install Go on your local development machine using this *[link](https://golang.org/doc/install)*. You can use a newer version of Go — there's no tie down to this specific version. We recommend using the newest stable version that you're comfortable with.

To learn more about the Go standardized project structure, please visit this [page](https://github.com/golang-standards/project-layout). As Golang's project structure differs quite significantly from other languages, this resource may prove to be helpful.

#### 3.2.2 Go Modules and Dependency Management System
Go's dependency management system might be a bit confusing for new developers. In Go, both packages and modules exist. A package in Go describes a directory of `.go` files, whereas a module describes a collection of Go packages that are related to each other. Go modules allow users to track and manage the module's dependency and versioning, as well as allows users to create local packages to use within the module. In any directory, you can create a go module like so:

```bash
$ cd <root-of-directory>
$ go mod init <module-name>
```

The command above will initialize the module in the directory and add a `go.mod` file, which tracks the name, version, and dependencies of the module. If you want to install a dependency to your module, you can either directly install like so:

```bash
$ cd <root-of-module>
$ go get <module-name>
```

Or you can import the package in your `.go` files with the assumption that it has already been installed, then run:
```bash
$ go mod tidy
```
This will automatically add any new dependencies into your `go.mod` file. Developers can choose to publish their modules to the public, and install modules from the public that they can use within their own module.
For this workshop and subsequent MapReduce workshops, a `mapreduce` module should be created in your local directory, with `master`, `worker` and other related packages within this. The `mapreduce` module will depend on public `grpc` and `etcd` modules. This will be done by the install script as noted in [section 6](#6-implementation) and [the repo specific instructions](#appendix-a--repo-specific-instructions). More information on go modules can be found *[here](https://levelup.gitconnected.com/using-modules-and-packages-in-go-36a418960556)*.

### 3.3 Useful References
- *[A Tour of Go](https://tour.golang.org/welcome/1)*
- *[Install Go](https://golang.org/doc/install)*
- *[Standard Go Project Layout](https://github.com/golang-standards/project-layout)*
- *[Go Modules](https://levelup.gitconnected.com/using-modules-and-packages-in-go-36a418960556)*
- *[Concurrency in Go by Katherine Cox-Buday](https://learning.oreilly.com/library/view/concurrency-in-go/9781491941294/)*
- *[C++ Tutorial](http://www.cplusplus.com/files/tutorial.pdf)*
- *[Thinking in C++ 2nd Edition by Bruce Eckel](https://archive.org/details/TICPP2ndEdVolOne)*
- *[Modern C++ by Scott Meyers](https://learning.oreilly.com/library/view/effective-modern-c/9781491908419/)*
- *[Kubernetes Concepts](https://kubernetes.io/docs/concepts/)*
- *[Kubernetes YAML video](https://www.youtube.com/watch?v=qmDzcu5uY1I&t=474s)*

## 4. Specification
Using Kubernetes, Docker, and etcd or ZooKeeper you are going to implement a Fault-Tolerant Master node on your local machine.

## 5. Download Repo and Installation
#### 5.0.1 C++ Users
Download C++ starter code using the following.
```bash
$ sudo apt-get update 
$ sudo apt-get install git
$ mkdir -p ~/src
$ cd ~/src
$ git clone https://github.gatech.edu/cs8803-SIC/workshop6-c.git
```

The downloaded git repository contains a bash script to install all the required dependencies. Run as follows:
```bash
$ cd ~/src/workshop6-c
$ chmod +x install.sh
$ ./install.sh
```

See [Appendix A](#appendix-a--repo-specific-instructions) for more comprehensive instructions.

#### 5.0.2 Go Users
Download Go starter code using the following.
```bash
$ sudo apt-get update 
$ sudo apt-get install git
$ mkdir -p ~/src
$ cd ~/src
$ git clone https://github.gatech.edu/cs8803-SIC/workshop6-go.git
```

The downloaded git repository contains a bash script to install all the required dependencies. Run as follows:
```bash
$ cd ~/src/workshop6
$ chmod +x install.sh
$ ./install.sh
```

See [Appendix A](#appendix-a--repo-specific-instructions) for more comprehensive instructions.

## 6. Implementation
This workshop has six steps which we will do in four phases:
1. Setting up C++ or Go Applications
2. Create the required data structures for the Master.
3. Use gRPC for creating the RPC calls.
4. Implement Leader Election with etcd or ZooKeeper
5. Building Containers
6. Deploying Containers with Kubernetes

### 6.1 Setting up a gRPC Application
The first thing you are going to do is create a simple gRPC application. Please follow the [C++ quickstart gRPC](https://grpc.io/docs/languages/cpp/quickstart/) or [Go quickstart gRPC](https://grpc.io/docs/languages/go/quickstart/) docs to guide your development.

Specifically, we would like you create the gRPC server in the `worker` node and put the gRPC client in the `master` node. The server in the worker node should receive a string and return the string with `gatech` appended to it. For example, if the input is `hello` , the server should return `hello gatech`. When the worker receives the call, it should log the input received using glog. There are a couple of these out there, just choose one that you like. 

If you are interested in learning how to setup your Go application directory structure in a standard way, please use this [link](https://github.com/golang-standards/project-layout).

Please test your binaries to make sure that they function correctly before moving onto the next section.

### 6.2 Implementing Leader Election
Next, once the gRPC server and client have been created and can successfully exchange information, you are going to implement leader election. If you have opted to use C++, you have the freedom to choose between using either etcd or ZooKeeper for your leader election implementation. If you're using Go, we strongly advise using etcd as it has been tried and tested. The teaching team will not be able to provide support to students using Go with ZooKeeper.

#### 6.2.1 Using etcd — C++ and Go
If you are using etcd, we recommend that you read about the API in the [C++ etcd docs](https://pkg.go.dev/github.com/coreos/etcd/clientv3/concurrency#Election) or [etcd docs](https://pkg.go.dev/github.com/coreos/etcd/clientv3/concurrency#Election), and follow [C++ blog posts](https://akazuko.medium.com/leader-election-using-etcd-10301473843c) or [Go blog posts](https://medium.com/@felipedutratine/leader-election-in-go-with-etcd-2ca8f3876d79) for how to implement leader election. Understand what is happening under the hood, as it may be discussed during the demo.

In addition to the master nodes, you should also think about how you are registering your worker nodes. You don't need to run an election for them, but saving some information in etcd might be a good idea. We recommend looking into etcd leases to potentially help with saving information for workers. Why could this be useful?

Unfortunately, at this point in time you will not be able to test your code unless you start a local etcd cluster. If you would like to make sure your leader election works before proceeding to the next section, be our guest! We are certain you will learn something by setting up etcd to run locally on your machine.

If using etcd with C++, use version *0.15.0* or prior. The correct behavior of callding `add()` with an existing key should cause a failure and the value should not be updated. Use version *0.15.0* or prior to have the correct behavior.

#### 6.2.2 Using ZooKeeper — C++ only
Selecting a leader is a really complex problem in distributed systems, luckily there are now frameworks that allow us to implement it more easily for certain scenarios (like this one). We are going to use ZooKeeper to implement the leader election. First, read about how ZooKeeper works in *[here](https://zookeeper.apache.org/doc/current/zookeeperProgrammers.html)*. An explanation (recipe) for implementing the leader election can be found *[here](https://zookeeper.apache.org/doc/current/recipes.html)*. The directory that is going to contain all the master nodes is going to be call `/master`.

ZooKeeper works as a standalone service. Your Master code should connect to it using the C Binding, to facilitate this we are going to use a C++ wrapper that was already installed in the previous script. The git for the repo can be found *[here](https://github.gatech.edu/cs8803-SIC/conservator.git)*. There are examples of how to use it (and compile it) in the directory `examples/`.

Once the leader is elected, then it needs to replicate each local state change to all the followers using RPC (to be implemented in following workshops). It is not until all the followers responded back that a state change can be committed.

We are also going to use ZooKeeper to keep a list of the available worker nodes, using ephemerals nodes in the directory `/workers` (what is an ephemeral node?). Additionally, to know which master replica to contact we are going to use the directory `/masters` and use the value with the lowest sequence (as explained in the Recipe). If there is a scenario in which the wrong master was contacted, it should reply back with and error and the address of the correct Master leader.

### 6.3 Building Containers
You can create your containers however you like, but we will give you hints on how to do it with Docker.
1. Create two Dockerfiles, `Dockerfile.master` and `Dockerfile.worker` ([Dockerfile Reference](https://docs.docker.com/engine/reference/builder/)).
2. Create a build script, `./build.sh`, in the root directory.
3. What does your build script need to do? Here are some suggestions:
    1. Generate your gRPC binaries
    2. Generate your application binaries
    3. Build your Docker images

#### 6.3.1 Building Containers with C++
Write this portion. It should contain information about statically or dynamically linking libraries in your containers.

In C++, the container setup will be a little more heavyweight than with Go. You have two choices to create C++ containers with your applications.

1. Dynamically Linking Applications (recommended)
2. Statically Linking Applications

The install script should build both the static libraries and dynamic libraries for all dependencies, including ZooKeeper and etcd. It is your choice whether you build a container with the static or dynamic libraries. Currently, it is recommended that you copy the necessary shared libraries into your container.

If you statically link your applications, it is not required to copy many if any libraries into your container.

### 6.4 Deploying Containers with Kubernetes
Now that you have the Docker images and binaries set up, it's time to build your Kubernetes application. As noted in [section 3](#3-background-information), you will be using KIND to set up a local Kubernetes cluster.

1. Get KIND set up.
2. Create a cluster.
3. Write a Kubernetes YAML Deployment file that will deploy 2 master pods and 1 worker pod (We recommend you using one service and two deployments)
4. Deploy your application to the local KIND cluster
5. Get logs for your pods.
Now is where the fun kicks in, it's time to start wrestling with Kubernetes. Useful commands can be found below:

    - Create a Kubernetes namespace to specify where things should be.
        ```bash
        $ kubectl create ns <your-namespace>
        ```

    - Use Helm to install etcd chart onto your cluster
        ```bash
        $ helm repo add bitnami https://charts.bitnami.com/bitnami
        $ helm install -n <your-namespace> etcd bitnami/etcd --set auth.rbac.create=false
        ```

    - Use Helm to install ZooKeeper chart onto your cluster
        ```bash
        $ helm repo add bitnami https://charts.bitnami.com/bitnami
        $ helm install <your-namespace> bitnami/zookeeper
        ```

    - Load Docker images to KIND
        ```bash
        $ kind load docker-image <your-master-image>
        $ kind load docker-image <your-worker-image>
        ```

    - Deploy your application
        ```bash
        $ kubectl -n <your-namespace> apply -f <your-kubernetes-configuration>.yaml 
        ```

    - Helpful `kubectl` commands:
        ```bash
        $ kubectl get all -n <your-namespace>
        $ kubectl -n <your-namespace> logs pod/<your-pod-id>
        $ kubectl -n <your-namespace> delete pod/<your-pod-id>
        ```

*Hints:* You may need to pass dynamic variables into your master, worker pod replicas (IP address, etcd/ZooKeeper endpoints information etc). You can do this by setting environmental variables of pods in your Kubernetes files and status files.

## 7. Useful References 
- [RPC Paper](https://github.com/papers-we-love/papers-we-love/blob/master/distributed_systems/a-note-on-distributed-computing.pdf)

## 8. Deliverables
Git repository with all the related code and a clear `README` file that explains how to compile and run the required scenarios. You are free to delete this `README.md` and write there. You will submit the repository and the commit id for the workshop in the comment section of the upload. You will continue using the same repository for the future workshops.


### 8.1 Demo
The demo for this workshop is as follows (to discuss with other students):

You should be able to demo leader election using `kubectl`. This means:

1. Initiate a master deployment with two pods. One of the masters should be elected as leader. Do an RPC call to the only worker with its address as the input. i.e. it should receive back `<address> gatech`.

2. Kill the current master abruptly. Now, the second master should become the new master and do an RPC call to the worker with its address as the input.

3. The initially dead master should be restarted and rejoin the election again. Kubernetes should automatically restart the initially killed master.

4. Log output should be relevant to the resign/rejoin of the master (i.e, print the hostname or podname unique to each master pod). Worker should properly log the input string along with the leader from which the request was received.

## Appendix A — Repo Specific Instructions
This appendix contains the original README for the repository. The original content is mostly covered by the workshop instructions. However, this section is included for convenience to have the basic instructions in one spot.

### Installation for C++
Run the `install.sh` script which will do the following:

1. Install kubectl
2. Install protobuf
3. Install KIND
4. Install Helm
5. Install gRPC
6. Set up conservator
7. Install etcd/ZooKeeper

Let the teaching team know if you have any issues.

### Directory Structure for C++ Repo
`CMakeLists.txt` will help you build your code. We recommend you read into cmake to understand how it works and how to use it to build your services.

The starter code for your master and worker files are in the `src/` directory, in the master and worker `.cc` files.

Happy Coding :3

### Installation for Go 
We assume you have installed Golang already on your system. For Golang installation instructions, visit [section 3.2](#32-golang) or their website [here](https://golang.org/doc/install). Run `install.sh` on your system after Golang has been installed. The install script should work for debian based distributions of linux.

The install script should do the following:

1. Install kubectl
2. Install protobuf
3. Install KIND
4. Install Helm
5. Install gRPC
6. Set up Go module with dependencies included

After set up, remember to add the path of Go and its libraries to the `PATH` environmental variable. Do this by pasting the following into `~/.bash_profile` or `~/.bashrc`, depending on your environment.

```bash
export PATH=$PATH:/usr/local/go/bin
export PATH="$PATH:$(go env GOPATH)/bin"
```

Let the teaching team know if you have any issues.

### Directory Structure for Go Repo
After installation, there should be a `go.mod` file. This defines the local go module for your implementation of MapReduce. The name of the module is defined as `mapreduce`, and the dependencies are defined in the `require` section.

The code for your master and worker files are in the `cmd/` directory, in the master and worker packages.

Happy Coding :3

