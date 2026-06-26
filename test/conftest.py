def pytest_configure(config):
    config.addinivalue_line(
        "markers", "images_integration"
    )
