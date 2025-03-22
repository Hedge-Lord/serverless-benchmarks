def main(params):
    """Simple test action to check OpenWhisk functionality"""
    import sys
    
    return {
        "status": "success",
        "message": "Hello from OpenWhisk!",
        "python_version": f"{sys.version_info.major}.{sys.version_info.minor}.{sys.version_info.micro}",
        "params_received": params
    } 