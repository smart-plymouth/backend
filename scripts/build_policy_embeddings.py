#!/usr/bin/env python3
"""Build the policy vector store from JLP and NPPF PDFs.

This script runs LOCALLY (not in the container) to:
1. Read the policy PDFs from data/policy/
2. Split them into chunks
3. Embed them using the nscale OpenAI-compatible API (Qwen3-Embedding-8B)
4. Persist the ChromaDB vector store to data/policy_vectorstore/

The resulting vector store directory is then committed to the repo and
baked into the Docker image so the application can query it at runtime
without needing access to the embedding model.

Usage:
    NSCALE_TOKEN=your-token python scripts/build_policy_embeddings.py

Requires:
    - NSCALE_TOKEN environment variable set
    - pip install chromadb langchain-chroma langchain-openai langchain pypdf
"""

import os
import shutil
import sys

from langchain_community.document_loaders import PyPDFLoader
from langchain_text_splitters import RecursiveCharacterTextSplitter
from langchain_openai import OpenAIEmbeddings
from langchain_chroma import Chroma

# Configuration
NSCALE_BASE_URL = os.environ.get("NSCALE_BASE_URL", "https://inference.api.nscale.com/v1")
NSCALE_TOKEN = os.environ.get("NSCALE_TOKEN", "")
EMBEDDING_MODEL = os.environ.get("EMBEDDING_MODEL", "Qwen/Qwen3-Embedding-8B")

BASE_DIR = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
POLICY_DIR = os.path.join(BASE_DIR, "data", "policy")
VECTORSTORE_DIR = os.path.join(BASE_DIR, "data", "policy_vectorstore")

POLICY_PDFS = [
    {
        "path": os.path.join(POLICY_DIR, "NPPF_December_2024.pdf"),
        "source": "NPPF",
        "description": "National Planning Policy Framework (December 2024)",
    },
    {
        "path": os.path.join(POLICY_DIR, "Plymouth_SW_Devon_JLP_2019.pdf"),
        "source": "JLP",
        "description": "Plymouth and South West Devon Joint Local Plan (2019)",
    },
]

# Chunking parameters
CHUNK_SIZE = 1000
CHUNK_OVERLAP = 200


def main():
    print("=" * 60)
    print("Building policy vector store")
    print("=" * 60)
    print(f"nscale URL: {NSCALE_BASE_URL}")
    print(f"Embedding model: {EMBEDDING_MODEL}")
    print(f"Policy dir: {POLICY_DIR}")
    print(f"Output dir: {VECTORSTORE_DIR}")
    print()

    if not NSCALE_TOKEN:
        print("ERROR: NSCALE_TOKEN environment variable is not set.")
        print("Usage: NSCALE_TOKEN=your-token python scripts/build_policy_embeddings.py")
        sys.exit(1)

    # Check PDFs exist
    for pdf_info in POLICY_PDFS:
        if not os.path.exists(pdf_info["path"]):
            print(f"ERROR: PDF not found: {pdf_info['path']}")
            sys.exit(1)
        size_mb = os.path.getsize(pdf_info["path"]) / (1024 * 1024)
        print(f"  Found: {pdf_info['source']} ({size_mb:.1f} MB)")

    # Clear existing vector store
    if os.path.exists(VECTORSTORE_DIR):
        print(f"\nRemoving existing vector store at {VECTORSTORE_DIR}")
        shutil.rmtree(VECTORSTORE_DIR)

    # Load and chunk all PDFs
    print("\nLoading and chunking PDFs...")
    text_splitter = RecursiveCharacterTextSplitter(
        chunk_size=CHUNK_SIZE,
        chunk_overlap=CHUNK_OVERLAP,
        separators=["\n\n", "\n", ". ", " ", ""],
    )

    all_documents = []
    for pdf_info in POLICY_PDFS:
        print(f"\n  Processing: {pdf_info['source']}...")
        loader = PyPDFLoader(pdf_info["path"])
        pages = loader.load()
        print(f"    Loaded {len(pages)} pages")

        # Add source metadata to each page
        for page in pages:
            page.metadata["source_document"] = pdf_info["source"]
            page.metadata["description"] = pdf_info["description"]

        chunks = text_splitter.split_documents(pages)
        print(f"    Split into {len(chunks)} chunks")
        all_documents.extend(chunks)

    print(f"\nTotal chunks: {len(all_documents)}")

    # Create embeddings and persist
    print(f"\nCreating embeddings using {EMBEDDING_MODEL}...")
    print("  (This may take several minutes for large documents)")

    embeddings = OpenAIEmbeddings(
        model=EMBEDDING_MODEL,
        base_url=NSCALE_BASE_URL,
        api_key=NSCALE_TOKEN,
    )

    # Build the vector store in batches to show progress
    batch_size = 100
    vectorstore = None

    for i in range(0, len(all_documents), batch_size):
        batch = all_documents[i:i + batch_size]
        batch_num = (i // batch_size) + 1
        total_batches = (len(all_documents) + batch_size - 1) // batch_size
        print(f"  Embedding batch {batch_num}/{total_batches} "
              f"({len(batch)} chunks)...")

        if vectorstore is None:
            vectorstore = Chroma.from_documents(
                documents=batch,
                embedding=embeddings,
                persist_directory=VECTORSTORE_DIR,
                collection_name="planning_policy",
            )
        else:
            vectorstore.add_documents(batch)

    print(f"\nVector store persisted to: {VECTORSTORE_DIR}")

    # Verify
    collection = vectorstore._collection
    print(f"Collection '{collection.name}' contains {collection.count()} embeddings")

    # Quick test query
    print("\nTest query: 'residential development design standards'")
    results = vectorstore.similarity_search(
        "residential development design standards", k=3
    )
    for i, doc in enumerate(results):
        source = doc.metadata.get("source_document", "?")
        page = doc.metadata.get("page", "?")
        print(f"  [{i+1}] {source} p.{page}: {doc.page_content[:80]}...")

    print("\n" + "=" * 60)
    print("Done! The vector store is ready to commit.")
    print(f"Directory: {VECTORSTORE_DIR}")
    print("=" * 60)


if __name__ == "__main__":
    main()
