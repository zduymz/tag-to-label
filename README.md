# tag-to-label

This project is one of many projects when i'm in a journey building k8s architecture for migrating to k8s from aws.

## Purpose
Using [Horizontal Pod Autoscaler](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/) and [Autoscaler](https://github.com/kubernetes/autoscaler/blob/master/cluster-autoscaler/cloudprovider/aws/README.md) is kind of pain if worker nodes had not labels. While waiting for this feature from K8S, I wrote this one as a temporary solution. Every worker nodes when join the K8S cluster will have the labels as its tag. 
   
To limit a number of redundant tags added. Checking tag prefix is added. If ec2 tag is `devops.apixio.com/hello` it will turn into a label `hello`  
## Testing on local
Edit `run` in `Makefile` to use correct configuration
```bash
# start minikube
minikube start

# build binary, because i used MacBook
make macos

# run to test
make run
```

## Run on Kubernetes
Worker Node must have at least the following IAM permissions
```json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Sid": "VisualEditor0",
            "Effect": "Allow",
            "Action": "ec2:Describe*",
            "Resource": "*"
        }
    ]
}
```
### Without RBAC
```bash
kubectl create -f manifest.yml
```

### With RBAC
```bash
kubectl create -f manifest-rbac.yml
```