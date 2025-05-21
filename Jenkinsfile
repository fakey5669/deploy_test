pipeline {
    agent any

    environment {
        // Preparation Stage
        WORK_DIR = "/home/k8scontrol"
        GITLAB_ADDR = "https://${GITLAB_ID}:${GITLAB_PW}@gitlab.mipllab.com/lw/workflow/k8scontrol.git"
        GITLAB_BRANCH = "main"

        // Build Stage
        HARBOR_ADDR = "harbor.mipllab.com -u ${HARBOR_ID} -p ${HARBOR_PW}"
        FRONTEND_IMG_TAG = "harbor.mipllab.com/lw/k8scontrol-frontend:latest"
        BACKEND_IMG_TAG = "harbor.mipllab.com/lw/k8scontrol-backend:latest"

        // Deployment Stage
        APP_PATH = "${WORK_DIR}/k8s/app.yaml"
        INGRESS_PATH = "${WORK_DIR}/k8s/ingress.yaml"
        NAMESPACE = "k8s-control"
        
        // ********* Safe Scripts **********
        SAFE_PREP = "/home/safe_preparation"
        SAFE_TERM = "/home/safe_termination"
    }

    stages {
        stage('Preparation') {
            steps {
                sh '''#!/bin/bash
                    mkdir -p ${WORK_DIR}
                    git -C ${WORK_DIR} init
                    git -C ${WORK_DIR} pull ${GITLAB_ADDR} ${GITLAB_BRANCH}
                '''
            }
        }

        stage('Preparation-SafetyCheck') {
            steps {
                sh '''#!/bin/bash
                    ${SAFE_PREP} ${WORK_DIR}
                '''
            }
        }

        stage('Build') {
            steps {
                sh '''#!/bin/bash
                    docker login ${HARBOR_ADDR}
                    cd ${WORK_DIR}
                    docker-compose build
                    docker push ${FRONTEND_IMG_TAG}
                    docker push ${BACKEND_IMG_TAG}
                '''
            }
        }

        stage('Deployment') {
            steps {
                sh '''#!/bin/bash
                    scp ${APP_PATH} tomcat@192.168.0.103:/home/tomcat/app.yaml
                    
                    ssh tomcat@192.168.0.103 "kubectl apply -f /home/tomcat/app.yaml -n ${NAMESPACE}"
                    
                    ssh tomcat@192.168.0.103 "kubectl rollout restart deployment/k8scontrol-frontend -n ${NAMESPACE}"
                    ssh tomcat@192.168.0.103 "kubectl rollout restart deployment/k8scontrol-backend -n ${NAMESPACE}"
                    
                    ssh tomcat@192.168.0.103 "rm /home/tomcat/app.yaml"
                '''
            }
        }

        stage('Termination') {
            steps {
                sh '''#!/bin/bash
                    rm -rf ${WORK_DIR}
                    docker rmi ${FRONTEND_IMG_TAG} ${BACKEND_IMG_TAG}
                '''
            }
        }

        stage('Termination-SafetyCheck') {
            steps {
                sh '''#!/bin/bash
                    ${SAFE_TERM} k8scontrol
                '''
            }
        }
    }

    post {
        failure {
            echo 'Pipeline failed!'
        }
        success {
            echo 'Pipeline succeeded!'
        }
    }
}
