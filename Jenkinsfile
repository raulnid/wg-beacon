// Builds the client image with kaniko and pushes it to Harbor (harbor-push secret lives in the jenkins ns)
pipeline {
  agent {
    kubernetes {
      yaml '''
apiVersion: v1
kind: Pod
spec:
  containers:
    - name: kaniko
      image: gcr.io/kaniko-project/executor:debug
      command: ["sleep"]
      args: ["infinity"]
      volumeMounts:
        - name: docker-config
          mountPath: /kaniko/.docker
        - name: homelab-root-ca
          mountPath: /kaniko/homelab-root-ca
  volumes:
    - name: homelab-root-ca
      secret:
        secretName: homelab-root-ca
    - name: docker-config
      secret:
        secretName: harbor-push
        items:
          - key: .dockerconfigjson
            path: config.json
'''
    }
  }
  environment {
    IMAGE = 'hub.xzr.es/library/wg-beacon'
  }
  stages {
    stage('build and push') {
      steps {
        container('kaniko') {
          // hub.xzr.es is served by the gateway with a step-ca cert, so trust the Homelab Root CA
          sh '''
            /kaniko/executor --context=dir://$WORKSPACE/client --dockerfile=Dockerfile \
              --destination=$IMAGE:${GIT_COMMIT:0:7} --destination=$IMAGE:latest \
              --registry-certificate=hub.xzr.es=/kaniko/homelab-root-ca/ca.crt
          '''
        }
      }
    }
  }
}
