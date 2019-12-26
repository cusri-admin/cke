该脚本通过检测需要unreserved的预留资源并且通过拼接/unreserve 接口所需的数据，最后通过post方法进行相应资源的unreserve

脚本检测逻辑：
首先从：5050端口的frameworks接口检测出该master节点上所有active和Inactive的framworks相应的framworkid并保存frameworkid的集合中
接着从：5050端口的slaves接口检测出所有带有labels,并且key="CKE-ID"的reserve信息，从“value”中提出相应frameworkid并保存到cke_frameworkid的集合中
最后对比两次保存的frameworkid,如果cke_frameworkid是frameworkid的子集，则没有需要unreserved的预留资源，
反之则通过对比出的frameworkid从slaves接口的reserved_resources_full数据中获取/unreserve所需的数据，通过post方法完成最后的unreserve

运行环境为;python3
脚本命令行参数：
-h --help 获取帮助介绍
-i --ip 输入相应的主机IP地址和端口

运行示例：
python3 unreserved_framwork.py -i 10.124.142.222:5050

部分主机python环境变量可能需要自己设置：
设置示例：
PYTHONPATH=/opt/mesosphere/lib/python3.6/site-packages