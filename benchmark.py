import asyncio
import aiohttp
import time
import random
import string
import argparse
from datetime import datetime

# API Configuration
BASE_URL = "http://localhost:8080"
DOMAIN = "shortenurl.org"

# Shared state for generated keys
populated_keys = []

async def do_post(session, metrics):
    url = f"https://example.com/{''.join(random.choices(string.ascii_letters, k=10))}"
    payload = {"domain": DOMAIN, "url": url}
    
    start = time.perf_counter()
    try:
        async with session.post(f"{BASE_URL}/newurl", json=payload) as resp:
            await resp.read()
            lat = time.perf_counter() - start
            if resp.status == 200:
                metrics['latencies'].append(lat)
                # Try to extract the shortKey for future reads
                try:
                    data = await resp.json()
                    short_url = data.get("shortenUrl", "")
                    short_key = short_url.split("/")[-1]
                    if short_key:
                        populated_keys.append(short_key)
                except:
                    pass
            else:
                metrics['errors'] += 1
    except Exception:
        metrics['errors'] += 1

async def do_get(session, metrics, key, expected_status=(200, 304)):
    start = time.perf_counter()
    try:
        # allow_redirects=False ensures we catch the 304/302 instead of following it
        async with session.get(f"{BASE_URL}/{key}", allow_redirects=False) as resp:
            await resp.read()
            lat = time.perf_counter() - start
            if resp.status in expected_status:
                metrics['latencies'].append(lat)
            else:
                metrics['errors'] += 1
    except Exception:
        metrics['errors'] += 1

async def worker(session, workload, duration, metrics):
    end_time = time.time() + duration
    while time.time() < end_time:
        if workload == 'A':
            await do_post(session, metrics)
        elif workload == 'B':
            if random.random() < 0.10:
                await do_post(session, metrics)
            else:
                key = random.choice(populated_keys) if populated_keys else "fallback"
                await do_get(session, metrics, key)
        elif workload == 'C':
            # Hot keys: pick from the first 100 generated keys
            pool = populated_keys[:100] if len(populated_keys) >= 100 else populated_keys
            key = random.choice(pool) if pool else "fallback"
            await do_get(session, metrics, key)
        elif workload == 'D':
            # Cold/Miss: Random 9 chars that likely don't exist
            key = ''.join(random.choices(string.ascii_letters + string.digits, k=9))
            await do_get(session, metrics, key, expected_status=(404,))

async def run_workload(name, desc, concurrency, duration, repeats):
    print(f"Starting {name}: {desc} ({repeats} runs, {duration}s each)...")
    all_runs = []
    
    # TCPConnector limits the connection pool size
    connector = aiohttp.TCPConnector(limit=concurrency)
    async with aiohttp.ClientSession(connector=connector) as session:
        for run in range(repeats):
            metrics = {'latencies': [], 'errors': 0}
            tasks = [worker(session, name[-1], duration, metrics) for _ in range(concurrency)]
            
            # Run warmup for 5 seconds on the first run of each workload
            if run == 0:
                warmup_metrics = {'latencies': [], 'errors': 0}
                warmup_tasks = [worker(session, name[-1], 5, warmup_metrics) for _ in range(concurrency)]
                await asyncio.gather(*warmup_tasks)
            
            await asyncio.gather(*tasks)
            all_runs.append(metrics)
            print(f"  Run {run+1}/{repeats} completed. Req: {len(metrics['latencies'])}, Err: {metrics['errors']}")
            
    return summarize_runs(name, desc, all_runs, duration)

def summarize_runs(name, desc, runs, duration):
    # Flatten all latencies across runs to get aggregate percentiles
    all_lats = []
    total_reqs = 0
    total_errs = 0
    for r in runs:
        all_lats.extend(r['latencies'])
        total_reqs += len(r['latencies'])
        total_errs += r['errors']
    
    if not all_lats:
        return f"| **{name}** | {desc} | {len(runs)}/{len(runs)} ok | 0.00 | N/A | N/A | N/A | N/A | N/A |"

    all_lats.sort()
    
    # Calculate microsecond (µs) metrics
    to_us = lambda s: int(s * 1_000_000)
    
    throughput = total_reqs / (duration * len(runs))
    p50 = to_us(all_lats[int(len(all_lats) * 0.50)])
    p90 = to_us(all_lats[int(len(all_lats) * 0.90)])
    p99 = to_us(all_lats[int(len(all_lats) * 0.99)])
    p999 = to_us(all_lats[int(len(all_lats) * 0.999)])
    max_lat = to_us(all_lats[-1])
    
    return f"| **{name}** | {desc} | {len(runs)}/{len(runs)} ok | {throughput:.2f} | {p50} | {p90} | {p99} | {p999} | {max_lat} |"

async def main():
    parser = argparse.ArgumentParser(description="URL Shortener YCSB-style Benchmark")
    parser.add_argument("-c", "--connections", type=int, default=100, help="Concurrent connections")
    parser.add_argument("-d", "--duration", type=int, default=60, help="Duration per run in seconds")
    parser.add_argument("-r", "--repeats", type=int, default=3, help="Number of repeats per workload")
    args = parser.parse_args()

    start_time = datetime.now()
    
    print("Pre-populating database with initial data (Workload A warmup)...")
    await run_workload("Workload A", "100% Write", args.connections, max(10, args.duration//4), 1)
    
    print(f"\nTarget Database populated with {len(populated_keys)} keys for read workloads.")
    
    results = []
    results.append(await run_workload("Workload A", "100% Write (POST /newurl)", args.connections, args.duration, args.repeats))
    results.append(await run_workload("Workload B", "90% Read / 10% Write", args.connections, args.duration, args.repeats))
    results.append(await run_workload("Workload C", "100% Read (Hot Keys)", args.connections, args.duration, args.repeats))
    results.append(await run_workload("Workload D", "100% Read (Cold/Miss)", args.connections, args.duration, args.repeats))

    end_time = datetime.now()
    wall_time = end_time - start_time

    print("\n\n" + "="*80)
    print("## URL Shortener Benchmark Report\n")
    print(f"**Generated:** {start_time.strftime('%Y-%m-%d %H:%M:%S')}")
    print(f"**Configuration:** Connections: {args.connections} | Duration per Workload: {args.duration}s | Repeats: {args.repeats} | Warmup: 5s")
    print(f"**Total wall time:** {wall_time}\n")
    print("### Results\n")
    print("| Workload | Description | Runs | Throughput (req/s) | p50 (µs) | p90 (µs) | p99 (µs) | p99.9 (µs) | Max (µs) |")
    print("| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |")
    for r in results:
        print(r)
    print("="*80 + "\n")

if __name__ == "__main__":
    # Required for Windows compatibility with asyncio
    import sys
    if sys.platform == 'win32':
        asyncio.set_event_loop_policy(asyncio.WindowsSelectorEventLoopPolicy())
    asyncio.run(main())
