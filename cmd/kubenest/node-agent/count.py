import sys
import time

if len(sys.argv) < 2:
    print("Usage: python a.py <number_of_iterations>")
    sys.exit(1)

try:
    iterations = int(sys.argv[1])
except ValueError:
    print("Error: The number of iterations should be an integer.")
    sys.exit(1)

for x in range(iterations):
    print(f"x = {x}")
    time.sleep(0.25)