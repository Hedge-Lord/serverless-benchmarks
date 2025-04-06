def main(args):
    """Simple web action for testing"""
    return {
        "body": {
            "success": True,
            "message": "Hello from Redis benchmark test action",
            "args": args
        }
    } 