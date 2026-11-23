package com.authrosetta.java.auth;

import java.util.Optional;
import java.util.UUID;

public interface UserRepository {
    /**
     * Finds a user by their normalized email.
     * @param normalizedEmail the normalized email of the user to find
     * @return an Optional containing the found user, or empty if no user is found
     */
    Optional<User> findByEmail(String normalizedEmail);

    /**
     * Finds a user by id.
     * @param id the id of the user to find
     * @return an Optional containing the found user, or empty if no user is found
     */
    Optional<User> findById(UUID id);
    
    /**
     * Adds a new user to the repository.
     * @param user the user to add
     * @return true if the user was added successfully, false otherwise
     */
    boolean add(User user);

    /**
     * Enables two-factor authentication for the user with the given id.
     * @param id the id of the user for whom to enable two-factor authentication
     */
    void enableTwoFactor(UUID id);
}
