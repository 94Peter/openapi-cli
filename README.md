# openapi-cli
將OpenAPI文件轉換為多種開發格式的命令行工具，以加速開發速度並簡化系統配置





## Usage

### docker

```cli
docker run -it \
	-v $$PWD:/workdir 94peter/openapi-cli:1.0 /main \
    ms -main /workdir/main_spec.yml \
    -mergeDir /workdir/allspec/ \
    -output /workdir/merged.yml
````