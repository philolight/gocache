# gocache
Separated lock and double buffering based lock avoiding cache written in golang

Main Feature
- By double buffering, can avoid lock between read and write (lock front for read, back for write)
- Separate lock to avoid massive read and write cache(https://stackoverflow.com/questions/10589103/concurrenthashmap-locking)
- Contains performance test(single map cache / bucket with single map / bucket with double buffering(gocache)


Concept
- single map cache
if N is number of elements,
map : |key1:val1||key2:val2|......|keyN:valN|
cons.:
    - reads are locking by writing
    - all reads are locking by any writing

- bucket with single map
if B is size of buckets,
bucket1 map : |key1:val1||key2:val2|......|key(N/B):val(N/B)|
bucket2 map : |key1:val1||key2:val2|......|key(N/B):val(N/B)|
......
bucketB map : |key1:val1||key2:val2|......|key(N/B):val(N/B)|

pros.:
    - locking probability are drop to 1/B by separated lock
cons.:
    - reads are locking by writing anyway

- bucket with double(gocache)
bucket1 map :   front |key1:val1||key2:val2|......|key(N/B):val(N/B)|
                back |key1:val1||key2:val2|......|key(N/B):val(N/B)|
bucket2 map :   front |key1:val1||key2:val2|......|key(N/B):val(N/B)|
                back |key1:val1||key2:val2|......|key(N/B):val(N/B)|
......
bucketB map :   front |key1:val1||key2:val2|......|key(N/B):val(N/B)|
                back |key1:val1||key2:val2|......|key(N/B):val(N/B)|

how to works :
    - front and back map has same elements
    - when you read, front lock works
    - when you write, back map write first, swap front and back, back map write again. front write lock occurs only when swap.
    - so read always works only excepts swap operation

pros.:
    - locking probability are drop to 1/B by separated lock
    - reads are locking only when front/back swap(only 3 instructions)