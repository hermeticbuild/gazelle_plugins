# Realistic Python file exercising the import forms the extractor handles.

import os
import sys
from pathlib import Path
from collections import OrderedDict, defaultdict
from typing import TYPE_CHECKING, Optional

import numpy as np
import pandas as pd
from sqlalchemy import create_engine
from sqlalchemy.orm import sessionmaker

from myorg.backend import settings
from myorg.backend.utils import format_name
from .local_helper import compute  # relative import
from ..parent_module import shared  # double-relative import

if TYPE_CHECKING:
    # type-checking-only — extractor flags these with type_checking_only=True.
    from myorg.backend.models import User, Session
    import boto3


def main() -> None:
    engine = create_engine(os.environ["DATABASE_URL"])
    Session = sessionmaker(bind=engine)
    df = pd.DataFrame(np.zeros((3, 3)))
    print(format_name(Path(sys.argv[0])), df, OrderedDict(), defaultdict(list))
    compute(shared)


def lazy_loader() -> Optional["bytes"]:
    # Function-scoped import — common pattern to defer expensive deps.
    import requests
    from urllib.parse import urlencode

    url = "https://example.com?" + urlencode({"q": "x"})
    return requests.get(url).content


def conditional_import():
    # Block-scoped import inside a try/except — used for optional deps.
    try:
        import orjson as json_lib
    except ImportError:
        import json as json_lib

    if sys.version_info >= (3, 11):
        from tomllib import loads as toml_loads
    else:
        from tomli import loads as toml_loads

    return json_lib, toml_loads


if __name__ == "__main__":
    main()
