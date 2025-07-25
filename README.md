# Distributed-Video-Storage

graph TB
    %% Client Layer
    Client[Client Browser]
    AdminCLI[Admin CLI]
    
    %% Web Server Layer
    WebServer[Web Server<br/>HTTP + gRPC]
    
    %% Service Interfaces
    subgraph "Service Layer"
        VMS[VideoMetadataService<br/>Interface]
        VCS[VideoContentService<br/>Interface]
    end
    
    %% Metadata Storage
    subgraph "Metadata Storage"
        direction TB
        subgraph "etcd Cluster (RAFT Consensus)"
            etcd1[etcd Node 1<br/>Leader]
            etcd2[etcd Node 2<br/>Follower]
            etcd3[etcd Node 3<br/>Follower]
        end
        
        EtcdService[EtcdVideoMetadataService<br/>Fault-Tolerant Metadata]
    end
    
    %% Content Storage
    subgraph "Content Storage"
        direction TB
        HashRing[Consistent Hash Ring<br/>SHA-256 + uint64]
        
        subgraph "Storage Cluster"
            Storage1[Storage Server 1]
            Storage2[Storage Server 2] 
            Storage3[Storage Server 3]
        end
        
        NWService[NetworkVideoContentService<br/>Distributed Storage]
    end
    
    %% Video Processing Pipeline
    subgraph "Video Processing"
        Upload[MP4 Upload]
        Convert[FFmpeg Conversion]
        Segments[MPEG-DASH<br/>manifest.mpd + .m4s]
        Distribute[Consistent Hash<br/>Distribution]
    end
    
    %% Client Interactions
    Client -->|GET /| WebServer
    Client -->|POST /upload| WebServer
    Client -->|GET /videos/:id| WebServer
    Client -->|GET /content/:id/:file| WebServer
    
    %% Admin Operations
    AdminCLI -->|gRPC: list/add/remove| WebServer
    
    %% Web Server to Services
    WebServer --> VMS
    WebServer --> VCS
    
    %% Metadata Connections
    VMS --> EtcdService
    EtcdService --> etcd1
    EtcdService --> etcd2
    EtcdService --> etcd3
    etcd1 -.->|RAFT| etcd2
    etcd2 -.->|RAFT| etcd3
    etcd3 -.->|RAFT| etcd1
    
    %% Content Storage Connections
    VCS --> NWService
    NWService --> HashRing
    HashRing --> Storage1
    HashRing --> Storage2
    HashRing --> Storage3
    
    %% Video Processing Flow
    Upload --> Convert
    Convert --> Segments
    Segments --> Distribute
    Distribute --> Storage1
    Distribute --> Storage2
    Distribute --> Storage3
    
    %% Storage Communication
    Storage1 -.->|gRPC| WebServer
    Storage2 -.->|gRPC| WebServer
    Storage3 -.->|gRPC| WebServer
    
    %% Styling
    classDef metadata fill:#e8f5e8
    classDef storage fill:#f3e5f5
    classDef interface fill:#fff3e0
    classDef processing fill:#e1f5fe
    
    class etcd1,etcd2,etcd3,EtcdService metadata
    class HashRing,NWService,Storage1,Storage2,Storage3 storage
    class VMS,VCS interface
    class Upload,Convert,Segments,Distribute processing
