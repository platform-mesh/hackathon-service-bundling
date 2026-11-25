# App Operator

Simple App Operator, which is bundled together with a database operator.

It watches `app.juiceshop.com` objects and creates the necessary database objects.

## Rough Whiteboard Drawing

```mermaid
flowchart LR
    User((User))

    subgraph kcp["kcp"]
        myWA["myWA"]
        myDB["myDB"]
        Secret["Secret"]
    end

    subgraph kube1["Kube 1"]
        WAOperator["WA-Operator"]
    end

    subgraph kube2["Kube 2"]
        DBOperator["DB-Operator"]
    end

    User -->|creates| `app.juiceshop.com`
    myWA -->|watches| WAOperator
    WAOperator -->|creates| myDB
    myDB -->|watches| DBOperator
    DBOperator -->|creates| Secret
```
