from pathlib import Path
from opensearchpy import OpenSearch

INPUT_FILE = Path("text.txt")
INDEX_NAME = "my_index"

client = OpenSearch(
    hosts=[{"host": "localhost", "port": 9200}],
)

def ingestData():
    with INPUT_FILE.open() as df:
        for line in df.readlines():
            client.index(index=INDEX_NAME, body={"stuff": line})
            print("Ingested: ", line)

if __name__ == "__main__":
    ingestData()
