# coding=utf-8
# by yangming 11/11 16:49

import json
import requests
from urllib import parse
import getopt
import sys

# 变量声明

# 从命令行获取的相应主机的IP地址和端口
url = ''
# frameworks接口的url
url_frameworks = ''
# slaves接口的url
url_slaves = ''
# 存储所有CKE-ID的frameworkid的集合
cke_frameworkid = set()
# 存储所有active以及inactive状态的frameworkid的集合
frameworkid = set()
# reservation中label的'key'
labels_key = 'CKE-ID'
# 存储需要unreserved的frameworkid集合
unreserved_id_set = set()
# 存储已经unreserved的CKE-ID
cke_id = set()

# request headers
headers = {
    'Cookie': "dcos-acs-auth-cookie=eyJhbGciOiJIUzI1NiJ9.eyIkaW50X3Blcm1zIjpbXSwic3ViIjoiY29tLmN1c2Z0LmRjb3MuYXV0aC51dGlscy5EY29zQXV0aFByb2ZpbGUjbnVsbCIsInVpZCI6ImFkbWluIiwiJGludF9yb2xlcyI6W10sImV4cCI6MTU3MzU0OTU0NywiZGlzcGxheV9uYW1lIjoiYWRtaW4iLCJpYXQiOjE1NzM1MjA3NDcsImVtYWlsIjoieGlhb3cxMEBjaGluYXVuaWNvbS5jbiIsImdyb3VwIjp7ImNvbW1vbk5hbWUiOiJtYW5hZ2VyIiwiZ210TW9kaWZpZWQiOiJUaHUgQXVnIDAxIDExOjExOjE2IEdNVCAyMDE5IiwiZ3JvdXBJZCI6bnVsbCwiZG9tYWluIjpudWxsLCJtZW1iZXIiOm51bGwsImRpc3Rpbmd1aXNoZWROYW1lIjpudWxsLCJkZXNjcmlwdGlvbiI6IueuoeeQhuWRmCIsImlkIjoxLCJnbXRDcmVhdGUiOiJUaHUgQXVnIDAxIDExOjExOjE2IEdNVCAyMDE5IiwicGFyZW50SWQiOm51bGwsImdycE1lbWJlciI6bnVsbH19.50uArPbCu-I41PqphVdMRc3sqVmpeaUfDV0FTQvOGjo",
}

# 获取CKE-ID中的frameworksID并且保存到cke_frameworkid中


def slaves_framework(jsons, key):

    cke_framework = ''
    index_key = 'value'

    if isinstance(jsons, dict):
        for json_data in jsons.values():
            if key in jsons.keys() and jsons.get(key) == labels_key:
                cke_framework = jsons.get(index_key)
            else:
                slaves_framework(json_data, key)
    elif isinstance(jsons, list):
        for json_array in jsons:
            slaves_framework(json_array, key)

    if cke_framework != '':
        # print(str(jsons.get(key)) + ' = ' + str(cke_framework))
        framework_id = str(cke_framework).split('.')[0]
        cke_frameworkid.add(framework_id)

# 获取active,connect,recovered状态为false的framworksid,并且保存到framworkid[]中


def get_frameworkid(jsons, key_id):

    for key, value in jsons.items():
        if key == 'frameworks':
            for index in value:
                if key_id in index.keys():
                    frameworkid.add(index.get(key_id))

# post unreserved方法体


def post_unreserve(data):

    url_unreserved = 'http://' + url + '/unreserve'
    response = requests.post(url_unreserved, data=data, headers=headers)
    print(response)


if __name__ == "__main__":

    try:
        options, args = getopt.getopt(sys.argv[1:], "hi:", ["help", "ip="])
    except getopt.GetoptError:
        print("参数输入有误 正确的输入命令行参数\n 例：-i 10.124.142.222:5050")
        sys.exit()
    # print(options)
    # print(args)
    if len(options) == 0:
        print("命令行参数不能为空\n正确的输入命令行参数例：-i 10.124.142.222:5050")
        exit()
    for option, value in options:
        if option in ("-h", "--help"):
            print("-h ,--help   该脚本为删除相应预留资源的脚本\n 命令行需要配置相应的主机IP和port\n -i --ip 配置主机IP地址和端口 例如：-i 10.124.142.222:5050\n")
        if option in ("-i", "--ip"):
            url = format(value)

    # get方法获取相应frameworks接口的数据
    url_frameworks = 'http://' + url + '/frameworks'
    res_frameworks = requests.get(url=url_frameworks, headers=headers)
    frameworks = json.loads(res_frameworks.text)
    # print(type(frameworks))
    # 调用方法并获取frameworkid的集合
    get_frameworkid(frameworks, 'id')

    # get方法获取相应slaves接口的数据
    url_slaves = 'http://' + url + '/slaves'
    res_slaves = requests.get(url=url_slaves, headers=headers)
    slaves = json.loads(res_slaves.text)['slaves']
    # 调用方法并获取cke-id的frameworks集合
    slaves_framework(slaves, 'key')
    print("CKE-ID中对应的framworkid:")
    print(cke_frameworkid)

    # 判断是否有需要unreserved的预留资源
    if cke_frameworkid.issubset(frameworkid) and cke_frameworkid != []:
        print("没有需要unreserved的预留资源")
    else:
        unregister_id_set = cke_frameworkid.difference(frameworkid)
        print("需要unreserved的资源对应frameworkid:")
        print(unregister_id_set)

        # 根据需要unreserved资源的framworksid 获取相应的unreserved post方法体
        for slave in slaves:
            unregister_resource_list = []
            slave_id = slave['id']
            # print(slave_id)
            for role in slave['reserved_resources_full']:
                for resource in slave['reserved_resources_full'][role]:
                    if 'reservation' not in resource or 'labels' not in resource['reservation'] or 'labels' not in resource['reservation']['labels']:
                        continue
                    flag = False
                    for label in resource['reservation']['labels']['labels']:
                        for key in label:
                            for unregister_id in unregister_id_set:
                                if unregister_id in label[key]:
                                    cke_id.add(label['value'])
                                    flag = True
                                    break
                            if flag:
                                break
                        if flag:
                            break
                    if flag:
                        unregister_resource_list.append({
                            'name': resource['name'],
                            'type': resource['type'],
                            'scalar': resource['scalar'],
                            'reservations': resource['reservations']
                        })
            if not unregister_resource_list:
                pass
            else:
                data = parse.urlencode({"resources": json.dumps(
                    unregister_resource_list), "slaveId": slave_id})
                # print(data)
                post_unreserve(data)
        print(cke_id)
