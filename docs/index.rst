beenet Documentation
====================

Welcome to beenet, a secure P2P networking library with Noise XX and Merkle trees.

.. toctree::
   :maxdepth: 2
   :caption: Contents:

Overview
--------

beenet is a Python library for secure peer-to-peer networking featuring:

* Noise XX secure channels for all peer traffic with mutual authentication
* Hybrid discovery using Kademlia DHT and BeeQuiet LAN protocol
* Chunked data transfer verified with Merkle trees
* Strong identity keys (Ed25519) with short-lived Noise static keys

Installation
------------

.. code-block:: bash

   pip install beenet

Quick Start
-----------

.. code-block:: python

   from beenet import Peer
   
   # Create a peer
   peer = Peer()
   
   # Start the peer
   await peer.start()

API Reference
-------------

.. automodule:: beenet
   :members:

Indices and tables
==================

* :ref:`genindex`
* :ref:`modindex`
* :ref:`search`
