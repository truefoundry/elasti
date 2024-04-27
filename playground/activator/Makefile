
docker-context:
	docker context use desktop-linux

docker-build: 
	docker build -t activator . 
	docker tag activator ramantehlan/activator

docker-publish:
	docker push ramantehlan/activator:latest

docker-registry:
	docker run -d -p 5000:5000 --name registry registry:2 

