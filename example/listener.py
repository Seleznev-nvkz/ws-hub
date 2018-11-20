import threading
from redis import StrictRedis


class RedisListener(threading.Thread):
    prefix = 'ws-hub'
    subscribe_channel = None

    def __init__(self, address: str = 'redis://localhost:6379/0'):
        threading.Thread.__init__(self)
        self._client = None
        self.address = address
        self.channel = f'{self.prefix}:{self.subscribe_channel}'

    @property
    def client(self):
        if self._client is None:
            self._client = StrictRedis.from_url(self.address)
        return self._client

    def run(self):
        raise NotImplementedError()


class NewClientListener(RedisListener):
    """ For subscribe client on many groups """
    subscribe_channel = 'client-new'

    def run(self):
        pubsub = self.client.pubsub()
        pubsub.subscribe(self.channel)
        for item in pubsub.listen():
            if item['type'] == 'message':
                data = item['data'].decode()
                print(f'New Client {data}')
                self.client.publish(f'{self.prefix}:groups-new:{data}', "1,2,all,news")


class DataClientListener(RedisListener):
    """ For receiving data from client """
    subscribe_channel = 'client-data:*'

    def run(self):
        pubsub = self.client.pubsub()
        pubsub.psubscribe(self.channel)
        for item in pubsub.listen():
            if item['type'] == 'pmessage':
                data = item['data'].decode()
                channel = item['channel'].decode()
                print(f"Data from {channel}:\n\t{data}")


if __name__ == "__main__":
    new_client_listener = NewClientListener()
    data_client_listener = DataClientListener()
    new_client_listener.start()
    data_client_listener.start()
