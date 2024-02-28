import pytest

from .ibc_utils import ANRYTON_IBC_DENOM, assert_ready, get_balance, prepare_network
from .utils import ADDRS, get_precompile_contract, wait_for_fn


@pytest.fixture(scope="module", params=[False])
def ibc(request, tmp_path_factory):
    "prepare-network"
    incentivized = request.param
    name = "stride-outpost"
    path = tmp_path_factory.mktemp(name)
    network = prepare_network(path, name, "stride", incentivized)
    yield from network


# TODO remove this test and replace with the outpost test
def test_ibc_transfer(ibc):
    """
    test transfer IBC precompile.
    """
    assert_ready(ibc)

    # stride chain is in ibc.orther_chain
    dst_addr = ibc.other_chain.cosmos_cli().address("signer2")
    amt = 1000000

    cli = ibc.anryton.cosmos_cli()
    src_addr = cli.address("signer2")
    src_denom = "anryton"

    old_src_balance = get_balance(ibc.anryton, src_addr, src_denom)
    old_dst_balance = get_balance(ibc.other_chain, dst_addr, ANRYTON_IBC_DENOM)

    pc = get_precompile_contract(ibc.anryton.w3, "ICS20I")
    anryton_gas_price = ibc.anryton.w3.eth.gas_price

    tx_hash = pc.functions.transfer(
        "transfer",
        "channel-0",
        src_denom,
        amt,
        ADDRS["signer2"],
        dst_addr,
        [1, 10000000000],
        0,
        "",
    ).transact({"from": ADDRS["signer2"], "gasPrice": anryton_gas_price})

    receipt = ibc.anryton.w3.eth.wait_for_transaction_receipt(tx_hash)

    assert receipt.status == 1
    # check gas used
    assert receipt.gasUsed == 127581

    fee = receipt.gasUsed * anryton_gas_price

    new_dst_balance = 0

    def check_balance_change():
        nonlocal new_dst_balance
        new_dst_balance = get_balance(ibc.other_chain, dst_addr, ANRYTON_IBC_DENOM)
        return old_dst_balance != new_dst_balance

    wait_for_fn("balance change", check_balance_change)
    assert old_dst_balance + amt == new_dst_balance
    new_src_balance = get_balance(ibc.anryton, src_addr, src_denom)
    assert old_src_balance - amt - fee == new_src_balance
