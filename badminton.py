import schedule
import os
import time

exec_cmd = """wget curl -X POST -H "Content-Type: application/json" -d '{"msg_type":"text","content":{"text":"badminton"}}'  https://open.feishu.cn/open-apis/bot/v2/hook/364e3121-db99-47bb-8984-ecc00ba15bed"""

def job():
    os.system(exec_cmd)

if __name__ == "__main__":
    schedule.every().monday.at("11:55").do(job)
    schedule.every().tuesday.at("11:55").do(job)
    schedule.every().wednesday.at("11:55").do(job)
    schedule.every().thursday.at("11:55").do(job)
    schedule.every().friday.at("11:55").do(job)
    while True:
        schedule.run_pending()
        time.sleep(5)
