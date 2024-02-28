def test_single_request_netversion(anryton):
    anryton.use_websocket()
    eth_ws = anryton.w3.provider

    response = eth_ws.make_request("net_version", [])

    # net_version should be 9000
    assert response["result"] == "9000", "got " + response["result"] + ", expected 9000"


# note:
# batch requests still not implemented in web3.py
# todo: follow https://github.com/ethereum/web3.py/issues/832, add tests when complete

# eth_subscribe and eth_unsubscribe support still not implemented in web3.py
# todo: follow https://github.com/ethereum/web3.py/issues/1402, add tests when complete


def test_batch_request_netversion(anryton):
    return


def test_ws_subscribe_log(anryton):
    return


def test_ws_subscribe_newheads(anryton):
    return
